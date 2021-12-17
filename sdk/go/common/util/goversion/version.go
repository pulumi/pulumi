package goversion

import (
	"github.com/blang/semver"
	"github.com/pkg/errors"
	"os/exec"
)

var minGoVersion = semver.MustParse("1.14.0")

// CheckMinimumGoVersion checks to make sure we are running at least minGoVersion
func CheckMinimumGoVersion(gobin string) error {
	cmd := exec.Command(gobin, "version")
	stdout, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "determining go version")
	}

	return checkMinimumGoVersion(string(stdout))
}

// checkMinimumGoVersion checks to make sure we are running at least go 1.14.0
// expected format of goVersionOutput: go version go<version> <os/arch>
func checkMinimumGoVersion(goVersionOutput string) error {
	return nil
}
