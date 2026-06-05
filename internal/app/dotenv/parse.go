package dotenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// mergeFiles parses files in dir in order and merges them, with later files overriding earlier values for matching keys
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

// isSafeRelativePath reports whether name is a relative path that stays inside its parent after cleaning
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

// parseFile reads a .env file and returns its key/value entries in declaration order, or nil on a read error
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

// parseLine parses a single .env line and returns ok=false for blanks, comments, and malformed lines
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
