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

// pulumi-pod-shim adapts a stock Pulumi plugin — which binds an ephemeral port on
// 127.0.0.1 and prints that port on its first line of stdout — to the simpler pod
// contract: a service reachable at a well-known port on all interfaces. The engine
// then dials the container at address:KNOWN and never scrapes a dynamic port, which
// removes the need for provider containers to share the engine's network namespace.
//
// It is a proxy, deliberately, not an iptables/route_localnet rebind: the proxy needs
// no elevated capabilities, bakes into any image as a single static binary (scratch and
// distroless included), and is a visible process rather than hidden kernel rules — in
// keeping with the "no hidden magic" ethos of the pod entrypoints.
//
// Usage:
//
//	PULUMI_POD_SHIM_PORT=7777 pulumi-pod-shim pulumi-resource-random
//
// Everything after the program name is the plugin command to exec. The shim scrapes the
// port the plugin prints, forwards 0.0.0.0:$PULUMI_POD_SHIM_PORT to 127.0.0.1:<scraped>,
// and re-emits the well-known port as its own handshake line — so it is a drop-in under
// both a host that still scrapes the port (dialing 127.0.0.1:KNOWN, which the 0.0.0.0
// listener covers) and a host that dials the container by address.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "pulumi-pod-shim: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	knownStr := os.Getenv("PULUMI_POD_SHIM_PORT")
	if knownStr == "" {
		return fmt.Errorf("PULUMI_POD_SHIM_PORT must be set (the well-known ingress port)")
	}
	known, err := strconv.Atoi(strings.TrimSpace(knownStr))
	if err != nil || known <= 0 || known > 65535 {
		return fmt.Errorf("PULUMI_POD_SHIM_PORT %q is not a valid port", knownStr)
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: pulumi-pod-shim <plugin> [args...]")
	}

	// Start the plugin. Its stderr is the container's stderr (logs flow through
	// untouched); its stdout we intercept to read the handshake port.
	//nolint:gosec // the wrapped command is supplied by the pod, not untrusted input
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.Stderr = os.Stderr

	// The shim exists to adapt a plugin that does NOT speak the bind contract. The engine
	// sets PULUMI_PLUGIN_LISTEN_ADDRESS on every provider container without knowing whether
	// the image is shim-wrapped; a wrapped plugin that honors it would bind the ingress port
	// first, making the shim's own bind — second — fatal. Strip the request so the wrapped
	// plugin always behaves like the stock loopback plugin the shim was built to adapt.
	cmd.Env = []string{}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PULUMI_PLUGIN_LISTEN_ADDRESS=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("wiring plugin stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting plugin %q: %w", os.Args[1], err)
	}

	// Relay termination signals to the plugin so it shuts down cleanly.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if s, ok := <-sigs; ok {
			_ = cmd.Process.Signal(s)
		}
	}()

	// Read the plugin's stdout up to the handshake line. Anything printed before the
	// port (diagnostics some plugins emit) is relayed to stderr so it is not mistaken
	// for the handshake; the remainder is drained to stderr once forwarding is up.
	scanner := bufio.NewScanner(stdout)
	pluginPort := 0
	for scanner.Scan() {
		line := scanner.Text()
		if p, ok := parsePort(line); ok {
			pluginPort = p
			break
		}
		fmt.Fprintln(os.Stderr, line)
	}
	if pluginPort == 0 {
		_ = cmd.Wait()
		return fmt.Errorf("plugin exited before printing a handshake port")
	}

	// Bind the well-known ingress port and forward it to the plugin's loopback port.
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", known))
	if err != nil {
		return fmt.Errorf("listening on ingress port %d: %w", known, err)
	}
	target := fmt.Sprintf("127.0.0.1:%d", pluginPort)
	fmt.Fprintf(os.Stderr, "pulumi-pod-shim: forwarding 0.0.0.0:%d -> %s\n", known, target)

	// Re-emit the well-known port as our handshake line so a host that still scrapes
	// stdout reads the ingress port, not the plugin's private one.
	fmt.Println(known)
	os.Stdout.Close()

	go acceptLoop(ln, target)
	go drainToStderr(scanner)

	err = cmd.Wait()
	_ = ln.Close()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("plugin: %w", err)
	}
	return nil
}

// acceptLoop forwards each inbound connection to the plugin's loopback port.
func acceptLoop(ln net.Listener, target string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed on plugin exit
		}
		go proxy(conn, target)
	}
}

// proxy splices one inbound connection to a fresh dial of the plugin.
func proxy(client net.Conn, target string) {
	defer client.Close()
	upstream, err := net.Dial("tcp", target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pulumi-pod-shim: dial %s: %v\n", target, err)
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

// drainToStderr relays any further plugin stdout (post-handshake diagnostics) so it is
// not lost now that our own stdout is closed.
func drainToStderr(scanner *bufio.Scanner) {
	for scanner.Scan() {
		fmt.Fprintln(os.Stderr, scanner.Text())
	}
}

// parsePort extracts a listen port from a plugin handshake line. The classic protocol
// prints a bare port number; be lenient and also accept a trailing host:port form.
func parsePort(line string) (int, bool) {
	line = strings.TrimSpace(line)
	if p, err := strconv.Atoi(line); err == nil && p > 0 && p <= 65535 {
		return p, true
	}
	if i := strings.LastIndex(line, ":"); i >= 0 {
		if p, err := strconv.Atoi(line[i+1:]); err == nil && p > 0 && p <= 65535 {
			return p, true
		}
	}
	return 0, false
}
