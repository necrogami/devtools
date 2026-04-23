// Package compose contains helpers for reading and mutating a project's
// .env file and docker-compose.yml without disturbing comments or formatting
// more than necessary.
package compose

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DevtoolsTagKey is the .env variable consumed by the compose template.
const DevtoolsTagKey = "DEVTOOLS_TAG"

// ProjectKey is the .env variable for the project/stack name.
const ProjectKey = "PROJECT"

// EnvFile returns the absolute path to a project's .env file.
func EnvFile(projectDir string) string {
	return filepath.Join(projectDir, ".env")
}

// ComposeFile returns the absolute path to a project's docker-compose.yml.
func ComposeFile(projectDir string) string {
	return filepath.Join(projectDir, "docker-compose.yml")
}

// ReadEnv returns the parsed key/value pairs from a .env file, preserving
// comments and blanks in the returned rawLines slice so writes can round-trip.
type EnvFileContent struct {
	path     string
	rawLines []string
	index    map[string]int // key → index into rawLines
}

// LoadEnv reads and parses path. Missing file returns an empty file ready
// for writing.
func LoadEnv(path string) (*EnvFileContent, error) {
	e := &EnvFileContent{path: path, index: map[string]int{}}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return e, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		e.rawLines = append(e.rawLines, line)
		if k, _, ok := parseAssignment(line); ok {
			e.index[k] = len(e.rawLines) - 1
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return e, nil
}

// Get returns the value for key and whether it was present.
func (e *EnvFileContent) Get(key string) (string, bool) {
	idx, ok := e.index[key]
	if !ok {
		return "", false
	}
	_, v, _ := parseAssignment(e.rawLines[idx])
	return v, true
}

// Set updates an existing key in place or appends a new assignment.
// Existing comments on the line are preserved only for updates-in-place.
func (e *EnvFileContent) Set(key, value string) {
	if idx, ok := e.index[key]; ok {
		e.rawLines[idx] = key + "=" + value
		return
	}
	e.rawLines = append(e.rawLines, key+"="+value)
	e.index[key] = len(e.rawLines) - 1
}

// Save writes the content back to disk, preserving trailing newline.
func (e *EnvFileContent) Save() error {
	var buf bytes.Buffer
	for _, line := range e.rawLines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return os.WriteFile(e.path, buf.Bytes(), 0o644)
}

// BumpTag sets DEVTOOLS_TAG in projectDir/.env to tag, creating the file
// if it doesn't yet exist. Returns the previous tag (or "" if unset).
func BumpTag(projectDir, tag string) (prev string, err error) {
	envPath := EnvFile(projectDir)
	env, err := LoadEnv(envPath)
	if err != nil {
		return "", err
	}
	prev, _ = env.Get(DevtoolsTagKey)
	env.Set(DevtoolsTagKey, tag)
	return prev, env.Save()
}

// parseAssignment parses `KEY=VALUE` from a line. Returns ok=false for
// comments, blank lines, or malformed lines.
func parseAssignment(line string) (key, value string, ok bool) {
	trim := strings.TrimSpace(line)
	if trim == "" || strings.HasPrefix(trim, "#") {
		return "", "", false
	}
	i := strings.IndexByte(line, '=')
	if i <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	value = line[i+1:]
	// Trim surrounding quotes if present.
	if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
		value[0] == '\'' && value[len(value)-1] == '\'') {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

// EnsureRequired confirms that .env declares the minimum required keys.
// Returns a descriptive error listing missing keys.
func EnsureRequired(projectDir string) error {
	env, err := LoadEnv(EnvFile(projectDir))
	if err != nil {
		return err
	}
	var missing []string
	for _, k := range []string{ProjectKey} {
		if v, ok := env.Get(k); !ok || v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s is missing required keys: %s",
			EnvFile(projectDir), strings.Join(missing, ", "))
	}
	return nil
}
