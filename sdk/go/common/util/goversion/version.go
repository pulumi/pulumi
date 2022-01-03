package goversion

import (
	"fmt"

	"github.com/blang/semver"

	"os/exec"
	"strings"
)

var minGoVersion = semver.MustParse("1.14.0")

// CheckMinimumGoVersion checks to make sure we are running at least minGoVersion
func CheckMinimumGoVersion(gobin string) error {
	cmd := exec.Command(gobin, "version")
	stdout, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("determining go version: %w", err)
	}

	return checkMinimumGoVersion(string(stdout))
}

// checkMinimumGoVersion checks to make sure we are running at least go 1.14.0
// expected format of goVersionOutput: go version go<version> <os/arch>
func checkMinimumGoVersion(goVersionOutput string) error {
	split := strings.Split(goVersionOutput, " ")
	if len(split) <= 2 {
		return fmt.Errorf("unexpected format for go version output: \"%s\"", goVersionOutput)

	}
	version := strings.TrimSpace(split[2])
	if strings.HasPrefix(version, "go") {
		version = version[2:]
	}

	currVersion, err := semver.ParseTolerant(version)
	if err != nil {
		return fmt.Errorf("parsing go version: %w", err)
	}

	if currVersion.LT(minGoVersion) {
		return fmt.Errorf("go version must be %s or higher (%s detected)", minGoVersion.String(), version)
	}
	return nil
}
