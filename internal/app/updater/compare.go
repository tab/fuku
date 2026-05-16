package updater

import (
	"strings"

	"golang.org/x/mod/semver"
)

// isNewer reports whether latest is a strictly newer semver release than current
func isNewer(current, latest string) bool {
	c := normalize(current)
	l := normalize(latest)

	if !semver.IsValid(c) || !semver.IsValid(l) {
		return false
	}

	return semver.Compare(l, c) > 0
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}

	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}

	return v
}
