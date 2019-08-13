// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		`^v(?P<version>\d+\.\d+\.\d+)(?P<dirty>\+dirty)?$`)
	rcVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-rc\.(?P<rcN>\d+)(?P<dirty>\+dirty)?$`)
	betaVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-beta\.(?P<betaN>\d+)(?P<dirty>.dirty)?$`)
	alphaVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-alpha\.(?P<time>\d+)\+(?P<gitInfo>g[a-z0-9]+)(?P<dirty>.dirty)?$`)
	devVersionRegex = regexp.MustCompile(
		`^v(?P<version>\d+\.\d+\.\d+)-dev\.(?P<time>\d+)\+(?P<gitInfo>g[a-z0-9]+)(?P<dirty>.dirty)?$`)
)

// PyPiVersionFromNpmVersion returns a PEP-440 compliant version for a given semver version. This method does not
// support all possible semver strings, but instead just supports versions that we generate for our node packages.
//
// NOTE: We do not include git information in the generated version (even within the local part, which PEP440 would
// allow) because we publish dev packages to PyPI, which does not allow local parts. Instead, we only add a local part
// when the build is dirty (which has the nice side effect of preventing us from publishing a build from dirty bits).
func PyPiVersionFromNpmVersion(s string) (string, error) {
	var b bytes.Buffer

	switch {
	case releaseVersionRegex.MatchString(s):
		capMap := captureToMap(releaseVersionRegex, s)
		mustFprintf(&b, "%s", capMap["version"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, "+dirty")
		}
		return b.String(), nil
	case rcVersionRegex.MatchString(s):
		capMap := captureToMap(rcVersionRegex, s)
		mustFprintf(&b, "%src%s", capMap["version"], capMap["rcN"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, "+dirty")
		}
		return b.String(), nil
	case betaVersionRegex.MatchString(s):
		capMap := captureToMap(betaVersionRegex, s)
		mustFprintf(&b, "%sb%s", capMap["version"], capMap["betaN"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, "+dirty")
		}
		return b.String(), nil
	case alphaVersionRegex.MatchString(s):
		capMap := captureToMap(alphaVersionRegex, s)
		mustFprintf(&b, "%sa%s", capMap["version"], capMap["time"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, "+dirty")
		}
		return b.String(), nil
	case devVersionRegex.MatchString(s):
		capMap := captureToMap(devVersionRegex, s)
		mustFprintf(&b, "%s.dev%s", capMap["version"], capMap["time"])
		if capMap["dirty"] != "" {
			mustFprintf(&b, "+dirty")
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

func mustFprintf(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.AssertNoError(err)
}
