// Copyright 2025, Pulumi Corporation.
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

// Package docker provides support for starting and stopping docker containers
// for running tests.
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// Container tracks information about the docker container started for tests.
type Container struct {
	ID       string
	Name     string
	HostPort string
}

// StartContainer starts the specified container for running tests.
func StartContainer(image string, name string, port string, dockerArgs []string, appArgs []string) (Container, error) {
	// When this code is used in tests, each test could be running in its own
	// process, so there is no way to serialize the call. The idea is to wait
	// for the container to exist if the code fails to start it.
	for i := range 2 {
		c, err := startContainer(image, name, port, dockerArgs, appArgs)
		if err != nil {
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
			continue
		}
		return c, nil
	}
	return startContainer(image, name, port, dockerArgs, appArgs)
}

// StopContainer stops and removes the specified container.
func StopContainer(id string) error {
	if err := exec.Command("docker", "stop", id).Run(); err != nil {
		return fmt.Errorf("could not stop container: %w", err)
	}
	if err := exec.Command("docker", "rm", id, "-v").Run(); err != nil {
		return fmt.Errorf("could not remove container: %w", err)
	}
	return nil
}

// DumpContainerLogs outputs logs from the running docker container.
func DumpContainerLogs(id string) []byte {
	out, err := exec.Command("docker", "logs", id).CombinedOutput()
	if err != nil {
		return nil
	}
	return out
}

// WaitForReady waits for the container to be ready by checking if a connection
// can be established to the specified address.
func WaitForReady(ctx context.Context, hostPort string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return errors.New("timeout waiting for container to be ready")
		default:
			conn, err := net.DialTimeout("tcp", hostPort, time.Second)
			if err == nil {
				conn.Close()
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// =============================================================================

func startContainer(image string, name string, port string, dockerArgs []string, appArgs []string) (Container, error) {
	if c, err := exists(name, port); err == nil {
		return c, nil
	}

	// Just in case there is a container with the same name.
	_ = exec.Command("docker", "rm", name, "-v").Run() // ignore error as container might not exist

	arg := []string{"run", "-P", "-d", "--name", name}
	arg = append(arg, dockerArgs...)
	arg = append(arg, image)
	arg = append(arg, appArgs...)

	var out bytes.Buffer
	cmd := exec.Command("docker", arg...)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return Container{}, fmt.Errorf("could not start container %s: %w", image, err)
	}

	id := out.String()[:12]
	hostIP, hostPort, err := extractIPPort(id, port)
	if err != nil {
		_ = StopContainer(id) // ignore error as original error is more important
		return Container{}, fmt.Errorf("could not extract ip/port: %w", err)
	}

	c := Container{
		ID:       id,
		Name:     name,
		HostPort: net.JoinHostPort(hostIP, hostPort),
	}

	return c, nil
}

func exists(name string, port string) (Container, error) {
	// Get the container ID first
	var idOut bytes.Buffer
	idCmd := exec.Command("docker", "ps", "-aqf", "name="+name) //nolint:gosec
	idCmd.Stdout = &idOut
	if err := idCmd.Run(); err != nil {
		return Container{}, errors.New("container not running")
	}

	id := strings.TrimSpace(idOut.String())
	if id == "" {
		return Container{}, errors.New("container not running")
	}

	hostIP, hostPort, err := extractIPPort(name, port)
	if err != nil {
		return Container{}, errors.New("container not running")
	}

	c := Container{
		ID:       id[:12],
		Name:     name,
		HostPort: net.JoinHostPort(hostIP, hostPort),
	}

	return c, nil
}

func extractIPPort(name string, port string) (hostIP string, hostPort string, err error) {
	// When IPv6 is turned on with Docker.
	// Got [{"HostIp":"0.0.0.0","HostPort":"49190"}{"HostIp":"::","HostPort":"49190"}]
	// Need [{"HostIp":"0.0.0.0","HostPort":"49190"},{"HostIp":"::","HostPort":"49190"}]
	// '[{{range $i,$v := (index .NetworkSettings.Ports "5432/tcp")}}{{if $i}},{{end}}{{json $v}}{{end}}]'
	tmpl := `[{{range $i,$v := (index .NetworkSettings.Ports "` + port +
		`/tcp")}}{{if $i}},{{end}}{{json $v}}{{end}}]`

	var out bytes.Buffer
	cmd := exec.Command("docker", "inspect", "-f", tmpl, name)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("could not inspect container %s: %w", name, err)
	}

	var docs []struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal(out.Bytes(), &docs); err != nil {
		return "", "", fmt.Errorf("could not decode json: %w", err)
	}

	for _, doc := range docs {
		if doc.HostIP != "::" {
			// Podman keeps HostIP empty instead of using 0.0.0.0.
			// - https://github.com/containers/podman/issues/17780
			if doc.HostIP == "" {
				return "localhost", doc.HostPort, nil
			}
			return doc.HostIP, doc.HostPort, nil
		}
	}

	return "", "", errors.New("could not locate ip/port")
}
