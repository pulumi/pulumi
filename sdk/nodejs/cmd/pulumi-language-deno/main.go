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

// This is the language plugin for Deno. The language host implementation is shared with
// pulumi-language-nodejs and selected with the `-runtime deno` argument.
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
	args = append(args, binary, "-runtime", "deno")
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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "error: failed to run pulumi-language-nodejs: %v\n", err)
		os.Exit(1)
	}
}

func findLanguageNodejs() (string, error) {
	program := "pulumi-language-nodejs"
	if runtime.GOOS == "windows" {
		program += ".exe"
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
			return "", fmt.Errorf("found %s but it is not executable", sideBySide)
		}
		return sideBySide, nil
	}
	if path, err := exec.LookPath(program); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not locate %s binary", program)
}
