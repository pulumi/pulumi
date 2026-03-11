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

// This is the language plugin for Deno. This shim around pulumi-language-nodejs exists so that the engine can load
// programs that have the runtime name `deno` specified. The actual LanguageRuntime RPC methods are implemented in
// pulumi-language-nodejs. pulumi-language-nodejs is started as a childprocess with the additional arguments
// `-runtime deno`. Signals are forwarded to this child process.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func main() {
	binary, err := findLanguageNodejs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to locate pulumi-language-nodejs: %v\n", err)
		os.Exit(1)
	}

	args := make([]string, 0, len(os.Args)+2)
	args = append(args, binary)
	args = append(args, "-runtime", "deno")
	args = append(args, os.Args[1:]...)

	cmd := &exec.Cmd{
		Path:   binary,
		Args:   args,
		Env:    os.Environ(),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to start pulumi-language-nodejs: %v\n", err)
		os.Exit(1)
	}

	// Forward signals to the child process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()

	waitErr := cmd.Wait()
	// Stop signal delivery and close the channel so the goroutine can exit cleanly.
	signal.Stop(sigChan)
	close(sigChan)

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "error: failed to run pulumi-language-nodejs: %v\n", waitErr)
		os.Exit(1)
	}
}

// findLanguageNodejs looks for `pulumi-language-nodejs` in the same directory as this binary first, then fall back to
// $PATH.
func findLanguageNodejs() (string, error) {
	program := "pulumi-language-nodejs"
	if runtime.GOOS == "windows" {
		program = "pulumi-language-nodejs.exe"
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return "", fmt.Errorf("could not resolve symlinks: %w", err)
	}
	sideBySide := filepath.Join(filepath.Dir(exePath), program)
	stat, err := os.Stat(sideBySide)
	if err == nil {
		if runtime.GOOS != "windows" && stat.Mode()&0o111 == 0 {
			fmt.Fprintf(os.Stderr, "warning: found %s but it is not executable; falling back to $PATH\n", sideBySide)
		} else {
			return sideBySide, nil
		}
	}

	if path, err := exec.LookPath(program); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("could not locate %s binary", program)
}
