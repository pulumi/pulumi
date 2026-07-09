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

package oci

import "os"

// Mode selects how the engine reaches a policy pack container's port.
type Mode int

const (
	// ModeHost is used when the CLI runs directly on a host: the container's
	// analyzer port is published to the host loopback interface.
	ModeHost Mode = iota
	// ModeSibling is used when the CLI itself runs inside a container with a
	// runtime socket (e.g. the Deployments executor): the pack container joins
	// the CLI container's network namespace and is dialed on loopback.
	ModeSibling
)

// EnvNetworkMode overrides networking mode detection ("host" or "sibling").
const EnvNetworkMode = "PULUMI_POLICY_CONTAINER_NETWORK"

// EnvSelfContainerID overrides the container ID used for sibling networking
// (defaults to the hostname, which container runtimes set to the container ID).
const EnvSelfContainerID = "PULUMI_SELF_CONTAINER_ID"

// DetectMode determines the networking mode for the current process.
func DetectMode() Mode {
	return detectMode(os.Getenv, func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	})
}

func detectMode(getenv func(string) string, fileExists func(string) bool) Mode {
	switch getenv(EnvNetworkMode) {
	case "host":
		return ModeHost
	case "sibling":
		return ModeSibling
	}
	inContainer := fileExists("/.dockerenv") || fileExists("/run/.containerenv")
	if !inContainer {
		return ModeHost
	}
	if getenv("DOCKER_HOST") != "" || fileExists("/var/run/docker.sock") {
		return ModeSibling
	}
	return ModeHost
}
