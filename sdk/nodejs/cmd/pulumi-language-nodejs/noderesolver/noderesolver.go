// Copyright 2026, Pulumi Corporation.
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

// Package noderesolver downloads and caches pinned Node.js distributions
// under ~/.pulumi/node/.
package noderesolver

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
)

type Spec struct {
	// Version is an exact Node.js version without a leading "v", e.g. "22.12.0".
	Version string
	// BaseURL overrides the download base URL, defaulting to
	// https://nodejs.org/dist. It must serve the nodejs.org/dist
	// directory layout.
	BaseURL string
	// Output, if non-nil, receives a one-line message when a download
	// actually starts. Nil means silent.
	Output io.Writer
}

type Result struct {
	Node   string
	Npm    string
	BinDir string
}

const defaultBaseURL = "https://nodejs.org/dist"

func (s Spec) baseURL() string {
	if s.BaseURL != "" {
		return strings.TrimRight(s.BaseURL, "/")
	}
	return defaultBaseURL
}

func archiveName(version, goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "linux", "darwin":
		osName = goos
	case "windows":
		osName = "win"
	default:
		return "", fmt.Errorf("no Node.js build available for OS %q (supported: linux, darwin, windows)", goos)
	}
	var archName string
	switch goarch {
	case "amd64":
		archName = "x64"
	case "arm64":
		archName = "arm64"
	default:
		return "", fmt.Errorf("no Node.js build available for architecture %q (supported: amd64, arm64)", goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("node-v%s-%s-%s%s", version, osName, archName, ext), nil
}

// layoutBinDir returns the directory containing the node executable inside an
// extracted distribution: Windows zips place binaries at the archive root,
// unix tarballs under bin/.
func layoutBinDir(root, archiveBase, goos string) string {
	if goos == "windows" {
		return filepath.Join(root, archiveBase)
	}
	return filepath.Join(root, archiveBase, "bin")
}

func nodeExe() string {
	if runtime.GOOS == "windows" {
		return "node.exe"
	}
	return "node"
}

func npmExe() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}
