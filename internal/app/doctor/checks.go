package doctor

import (
	"os"
	"time"
)

// timed records the duration of fn into the returned result
func timed(fn func() Result) Result {
	start := time.Now()
	r := fn()
	r.DurationMs = time.Since(start).Milliseconds()

	return r
}

// fileExists reports whether path refers to an existing file
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirExists reports whether path exists and is a directory
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
