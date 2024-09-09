// Copyright 2020-2024, Pulumi Corporation.
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

package goversion

import (
	"fmt"
	"os/exec"
	"strings"

	goVersion "github.com/hashicorp/go-version"
)

var minGoVersion = goVersion.Must(goVersion.NewVersion("1.14.0"))

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
	version = strings.TrimPrefix(version, "go")

	currVersion, err := goVersion.NewVersion(version)
	if err != nil {
		return fmt.Errorf("parsing go version: %w", err)
	}

	if currVersion.LessThan(minGoVersion) {
		return fmt.Errorf("go version must be %s or higher (%s detected)", minGoVersion.String(), version)
	}
	return nil
}
