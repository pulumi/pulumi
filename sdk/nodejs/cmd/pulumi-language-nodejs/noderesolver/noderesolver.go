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

// Package noderesolver locates a node/npm toolchain: ambient PATH first, falling
// back to a pinned Node.js distribution downloaded into ~/.pulumi/node/.
package noderesolver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Spec struct {
	// Version is the Node.js version without a leading "v", e.g. "24.18.0".
	Version   string
	BaseURL   string
	Checksums map[string]string

	// Disabled disables the managed fallback: when ambient node is missing,
	// ResolveWith returns an actionable error instead of downloading one.
	Disabled bool

	// Output, if non-nil, receives a one-line message when a download
	// actually starts. Nil means silent.
	Output io.Writer
}

type Result struct {
	Node    string
	Npm     string
	BinDir  string
	Managed bool
}

const defaultBaseURL = "https://nodejs.org/dist"

func Default() Spec {
	base := defaultBaseURL
	if v := os.Getenv("PULUMI_NODE_DOWNLOAD_URL"); v != "" {
		base = strings.TrimRight(v, "/")
	}
	disabled := false
	if v := os.Getenv("PULUMI_DISABLE_MANAGED_NODE"); v == "true" || v == "1" {
		disabled = true
	}
	return Spec{Version: PinnedVersion, BaseURL: base, Checksums: pinnedChecksums, Disabled: disabled}
}

func archiveFile(spec Spec, goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "linux", "darwin":
		osName = goos
	case "windows":
		osName = "win"
	default:
		return "", fmt.Errorf("no managed Node.js build for OS %q", goos)
	}
	var archName string
	switch goarch {
	case "amd64":
		archName = "x64"
	case "arm64":
		archName = "arm64"
	default:
		return "", fmt.Errorf("no managed Node.js build for architecture %q", goarch)
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("node-v%s-%s-%s%s", spec.Version, osName, archName, ext)
	if _, ok := spec.Checksums[name]; !ok {
		return "", fmt.Errorf("no managed Node.js build for %s/%s", goos, goarch)
	}
	return name, nil
}

func layoutBinDir(root, archiveBase, goos string) string {
	if goos == "windows" {
		return filepath.Join(root, archiveBase)
	}
	return filepath.Join(root, archiveBase, "bin")
}
