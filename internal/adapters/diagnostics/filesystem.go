package diagnostics

import "os"

// Filesystem observes the working directory and the paths a project references
type Filesystem struct{}

// NewFilesystem creates an observer of the filesystem
func NewFilesystem() *Filesystem {
	return &Filesystem{}
}

// Getwd returns the current working directory
func (f *Filesystem) Getwd() (string, error) {
	return os.Getwd()
}

// FileExists reports whether path refers to an existing file
func (f *Filesystem) FileExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && !info.IsDir()
}

// DirExists reports whether path exists and is a directory
func (f *Filesystem) DirExists(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}
