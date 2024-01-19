package sdk

import (
	_ "embed"
	"strings"

	"github.com/blang/semver"
)

//go:embed .version
var version string

// Version is the SDK's release version. Be aware that if the SDK is installed
// from a git commit, this will refer to the next proposed release, not a
// format like "1.2.3-$COMMIT".
var Version semver.Version

func init() {
	Version = semver.MustParse(strings.TrimSpace(version))
}
