package goversion

import (
	goVersion "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"os/exec"
	"strings"
)

var minGoVersion = goVersion.Must(goVersion.NewVersion("1.14.0"))

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
	split := strings.Split(goVersionOutput, " ")
	if len(split) <= 2 {
		return errors.Errorf("unexpected format for go version output: \"%s\"", goVersionOutput)

	}
	version := strings.TrimSpace(split[2])
	if strings.HasPrefix(version, "go") {
		version = version[2:]
	}

	currVersion, err := goVersion.NewVersion(version)
	if err != nil {
		return errors.Wrap(err, "parsing go version")
	}

	if currVersion.LessThan(minGoVersion) {
		return errors.Errorf("go version must be %s or higher (%s detected)", minGoVersion.String(), version)
	}
	return nil
}
