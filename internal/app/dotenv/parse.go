package dotenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// mergeFiles parses files in dir in the given order; later files override earlier values for matching keys; output preserves first-appearance order of keys; entries with absolute paths or `..` segments are skipped to prevent reading files outside dir
func mergeFiles(dir string, files []string) []Store {
	if dir == "" || len(files) == 0 {
		return nil
	}

	indices := make(map[string]int)

	var result []Store

	for _, name := range files {
		if !isSafeRelativePath(name) {
			continue
		}

		path := filepath.Join(dir, name)

		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}

		for _, e := range parseFile(path) {
			idx, exists := indices[e.Key]
			if exists {
				result[idx].Value = e.Value

				continue
			}

			indices[e.Key] = len(result)
			result = append(result, Store{Key: e.Key, Value: e.Value})
		}
	}

	return result
}

// isSafeRelativePath reports whether name is a relative path that resolves inside its parent directory; absolute paths and paths with `..` segments are rejected so a misconfigured env.files entry cannot read files outside the service directory
func isSafeRelativePath(name string) bool {
	if name == "" || filepath.IsAbs(name) {
		return false
	}

	cleaned := filepath.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}

// parseFile reads a .env* file and returns its key/value entries in declaration order; entries are returned only when the scan completes cleanly so a mid-scan I/O error does not yield a partial set; the caller must restrict path to a service-relative file already validated by isSafeRelativePath
func parseFile(path string) []Store {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}

	defer func() { _ = f.Close() }()

	var entries []Store

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if kv, ok := parseLine(scanner.Text()); ok {
			entries = append(entries, kv)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil
	}

	return entries
}

// parseLine parses a single line; returns ok=false for blanks, comments, and malformed lines
func parseLine(line string) (Store, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return Store{}, false
	}

	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))

	idx := strings.IndexByte(trimmed, '=')
	if idx <= 0 {
		return Store{}, false
	}

	key := strings.TrimSpace(trimmed[:idx])
	if key == "" {
		return Store{}, false
	}

	return Store{Key: key, Value: trimmed[idx+1:]}, true
}
