package version

import (
	"strings"
)

// BuildVersion is injected at compile time via ldflags
// Example: go build -ldflags "-X github.com/TIANLI0/BS2PRO-Controller/internal/version.BuildVersion=2.1.0"
var BuildVersion = "dev"

// Get returns the application version number
// Prefers the version injected at compile time; returns "dev" if not injected
func Get() string {
	if v := strings.TrimSpace(BuildVersion); v != "" {
		return v
	}
	return "dev"
}
