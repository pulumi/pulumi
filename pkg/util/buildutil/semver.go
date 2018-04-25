package buildutil

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

var (
	releaseVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)(-(?P<time>\d+)-(?P<gitInfo>g[a-z0-9]+))?(?P<dirty>-dirty)?$`)
	rcVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-rc(?P<rcN>\d+)(-(?P<time>\d+)-(?P<gitInfo>g[a-z0-9]+))?(?P<dirty>-dirty)?$`)
	devVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-dev-(?P<time>\d+)-(?P<gitInfo>g[a-z0-9]+)(?P<dirty>-dirty)?$`)
)

// PyPiVersionFromNpmVersion returns a PEP-440 compliant version for a given semver version. This method does not
// support all possible semver strings, but instead just supports versions that we generate for our node packages.
func PyPiVersionFromNpmVersion(s string) (string, error) {
	var b bytes.Buffer

	if releaseVersionRegex.MatchString(s) {
		capMap := captureToMap(releaseVersionRegex, s)
		mustFprintf(&b, "%s", capMap["version"])
		writePostBuildAndDirtyInfoToReleaseVersion(&b, capMap)
		return b.String(), nil

	} else if rcVersionRegex.MatchString(s) {
		capMap := captureToMap(rcVersionRegex, s)
		mustFprintf(&b, "%src%s", capMap["version"], capMap["rcN"])
		writePostBuildAndDirtyInfoToReleaseVersion(&b, capMap)
		return b.String(), nil
	} else if devVersionRegex.MatchString(s) {
		capMap := captureToMap(devVersionRegex, s)
		mustFprintf(&b, "%s.dev%s+%s", capMap["version"], capMap["time"], capMap["gitInfo"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, ".dirty")
		}
		return b.String(), nil
	}

	return "", errors.Errorf("can not parse version string '%s'", s)
}

func captureToMap(r *regexp.Regexp, s string) map[string]string {
	matches := r.FindStringSubmatch(s)
	capMap := make(map[string]string)
	for i, name := range r.SubexpNames() {
		if name != "" {
			capMap[name] = matches[i]
		}
	}

	return capMap
}

// While the version string for dev builds always contain timestamp and commit information, release and release
// release candidate builds do not. In the case where we do have this information, it is for a build newer than
// the actual release build, and we'll use the PEP-440 .post notation to show this. We also handle adding the dirty
// tag in the local version if we need it.
func writePostBuildAndDirtyInfoToReleaseVersion(w io.Writer, capMap map[string]string) {
	if capMap["time"] != "" {
		mustFprintf(w, ".post%s", capMap["time"])
		mustFprintf(w, "+%s", capMap["gitInfo"])
		if capMap["dirty"] != "" {
			mustFprintf(w, ".dirty")
		}
	} else if capMap["dirty"] != "" {
		mustFprintf(w, "+dirty")
	}
}

func mustFprintf(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.AssertNoError(err)
}
