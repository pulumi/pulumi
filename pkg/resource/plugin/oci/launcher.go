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

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// ContainerPort is the fixed port the pack's analyzer listens on inside the
// container network namespace in host mode (communicated via EnvPolicyPort).
const ContainerPort = 20851

// EnvPolicyPort tells a policy pack which port to serve its analyzer on.
const EnvPolicyPort = "PULUMI_POLICY_PORT"

// LabelKey labels containers launched for policy packs.
const LabelKey = "com.pulumi.policy-pack"

// LaunchOptions configures a policy pack container launch.
type LaunchOptions struct {
	Image           string            // image ref to run (never pulled implicitly)
	PackName        string            // for container naming/labels and errors
	Env             map[string]string // additional environment for the pack
	Mode            Mode
	SelfContainerID string // sibling mode: container to share a netns with; defaults to hostname
}

// Container is a running policy pack container.
type Container struct {
	rt   *Runtime
	id   string
	Addr string // host-reachable analyzer address, "127.0.0.1:<port>"
}

// run executes the runtime CLI, returning combined output.
func (r *Runtime) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.Path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", r.Name, strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func sanitizeName(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Launch starts the pack container and returns it with a dialable address.
// The image is never pulled implicitly (--pull=never): local packs must be
// built first, and enforced packs are pulled at install time.
func (r *Runtime) Launch(ctx context.Context, opts LaunchOptions) (*Container, error) {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	name := fmt.Sprintf("pulumi-policy-%s-%s", sanitizeName(opts.PackName), hex.EncodeToString(suffix))

	args := []string{
		"run", "--detach", "--rm", "--pull=never",
		"--name", name,
		"--label", LabelKey + "=" + sanitizeName(opts.PackName),
	}

	var dialPort int
	switch opts.Mode {
	case ModeHost:
		args = append(args,
			"-e", fmt.Sprintf("%s=%d", EnvPolicyPort, ContainerPort),
			"-p", fmt.Sprintf("127.0.0.1:0:%d", ContainerPort),
			"--add-host=host.docker.internal:host-gateway",
		)
	case ModeSibling:
		port, err := freePort()
		if err != nil {
			return nil, fmt.Errorf("finding a free port for policy pack %q: %w", opts.PackName, err)
		}
		dialPort = port
		self := opts.SelfContainerID
		if self == "" {
			self = os.Getenv(EnvSelfContainerID)
		}
		if self == "" {
			self, _ = os.Hostname()
		}
		args = append(args,
			"-e", fmt.Sprintf("%s=%d", EnvPolicyPort, port),
			"--network", "container:"+self,
		)
	}

	keys := make([]string, 0, len(opts.Env))
	for k := range opts.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+opts.Env[k])
	}

	args = append(args, opts.Image)

	id, err := r.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to start container for policy pack %q (image %s): %w",
			opts.PackName, opts.Image, err)
	}

	c := &Container{rt: r, id: id}
	if opts.Mode == ModeHost {
		out, err := r.run(ctx, "port", id, strconv.Itoa(ContainerPort))
		if err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("discovering mapped port for policy pack %q: %w", opts.PackName, err)
		}
		// Output may span multiple lines (ipv4+ipv6); take the first.
		c.Addr = strings.TrimSpace(strings.Split(out, "\n")[0])
		// nerdctl/podman may print "0.0.0.0:49321"; dial loopback regardless.
		if _, port, splitErr := net.SplitHostPort(c.Addr); splitErr == nil {
			c.Addr = net.JoinHostPort("127.0.0.1", port)
		}
	} else {
		c.Addr = fmt.Sprintf("127.0.0.1:%d", dialPort)
	}
	return c, nil
}

// Running reports whether the container is still running.
func (c *Container) Running(ctx context.Context) bool {
	out, err := c.rt.run(ctx, "inspect", "--format", "{{.State.Running}}", c.id)
	return err == nil && strings.TrimSpace(out) == "true"
}

// Logs returns the container's combined logs (best-effort).
func (c *Container) Logs(ctx context.Context) string {
	out, err := c.rt.run(ctx, "logs", c.id)
	if err != nil {
		return fmt.Sprintf("(could not fetch container logs: %v)", err)
	}
	return out
}

// Close stops the container. The container was started with --rm, so stopping
// also removes it. A missing container (already exited) is not an error.
func (c *Container) Close() error {
	_, err := c.rt.run(context.Background(), "stop", "--time", "2", c.id)
	if err != nil && strings.Contains(err.Error(), "No such container") {
		return nil
	}
	return err
}

// EngineAddressFor rewrites the engine's gRPC address for reachability from
// inside a pack container. In host mode the container cannot reach the host's
// loopback, so the host is rewritten to host.docker.internal (mapped via
// --add-host=host-gateway at launch). In sibling/attach modes the namespace is
// shared and the address passes through unchanged.
func EngineAddressFor(mode Mode, engineAddr string) string {
	if mode != ModeHost {
		return engineAddr
	}
	_, port, err := net.SplitHostPort(engineAddr)
	if err != nil {
		return engineAddr
	}
	return net.JoinHostPort("host.docker.internal", port)
}
