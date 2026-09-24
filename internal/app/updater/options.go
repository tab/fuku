package updater

// Options carries the running version and whether the release check is enabled
type Options struct {
	Enabled bool
	Version string
}
