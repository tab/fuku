package envfiles

import (
	"bufio"
	"io"
	"strings"

	"fuku/internal/model"
)

// parse returns the key/value entries of a .env stream in declaration order
func parse(r io.Reader) ([]model.Env, error) {
	var entries []model.Env

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if kv, ok := parseLine(scanner.Text()); ok {
			entries = append(entries, kv)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// parseLine parses a single .env line and returns ok=false for blanks, comments, and malformed lines
func parseLine(line string) (model.Env, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return model.Env{}, false
	}

	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))

	idx := strings.IndexByte(trimmed, '=')
	if idx <= 0 {
		return model.Env{}, false
	}

	return model.Env{Key: strings.TrimSpace(trimmed[:idx]), Value: trimmed[idx+1:]}, true
}
