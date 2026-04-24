// Package tmpl copies the project template directory into a destination,
// rendering Go template syntax in text files along the way.
package tmpl

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Vars are injected into {{.Field}} placeholders inside template files.
type Vars struct {
	Project     string
	DevtoolsTag string
}

// Render copies every file from srcDir to dstDir.
//
// Files whose name ends in one of the renderExts extensions have their
// contents rendered through text/template using vars. Files ending in
// ".example" are copied with that suffix stripped, so .env.example →
// .env (etc.) — this is how `dev new` turns the template into a live
// project stack.
//
// Returns an error if dstDir already exists (we do not overwrite).
func Render(srcDir, dstDir string, vars Vars) error {
	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("destination already exists: %s", dstDir)
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstDir, 0o755)
		}

		target := filepath.Join(dstDir, stripExampleSuffix(rel))

		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target, vars)
	})
}

// stripExampleSuffix converts ".env.example" → ".env" and "Brewfile.example"
// → "Brewfile" so the rendered project directly picks up its real config.
func stripExampleSuffix(path string) string {
	return strings.TrimSuffix(path, ".example")
}

// renderExts lists file extensions that get template-expanded. Binary files
// and unknown extensions are copied byte-for-byte.
var renderExts = map[string]bool{
	".md":     true,
	".yml":    true,
	".yaml":   true,
	".toml":   true,
	".env":    true,
	".sh":     true,
	".json":   true,
	"":        true, // dotfiles with no extension (e.g., README)
	".example": true,
}

func copyFile(src, dst string, vars Vars) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if shouldRender(src) {
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		tmpl, err := template.New(filepath.Base(src)).
			Option("missingkey=error").
			Parse(string(data))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", src, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, vars); err != nil {
			return fmt.Errorf("render template %s: %w", src, err)
		}
		_, err = out.Write(buf.Bytes())
		return err
	}

	_, err = io.Copy(out, in)
	return err
}

func shouldRender(path string) bool {
	// Pick up the final extension; handles compound like .env.example → .example.
	ext := filepath.Ext(path)
	if renderExts[ext] {
		return true
	}
	// Files with two extensions (foo.env.example) should respect the
	// MIDDLE ext for rendering decisions.
	trimmed := strings.TrimSuffix(path, ext)
	inner := filepath.Ext(trimmed)
	return renderExts[inner]
}
