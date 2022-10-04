// Copyright 2016-2022, Pulumi Corporation.
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

package rpcutil

import (
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type ptyCloser struct {
	done     chan (error)
	pty, tty *os.File
}

func (w *ptyCloser) Close() error {
	// Close can be called multiple times, but we will of nil'd out everything first time.
	if w.done == nil {
		contract.Assert(w.pty == nil)
		contract.Assert(w.tty == nil)
		return nil
	}

	// Try to close the tty
	terr := w.tty.Close()
	// Wait for the done signal
	err := <-w.done
	// Now close the pty
	perr := w.pty.Close()

	// if err is an error because pty closed ignore it
	if ioErr, ok := err.(*fs.PathError); ok {
		if sysErr, ok := ioErr.Err.(syscall.Errno); ok {
			if sysErr == syscall.EIO {
				err = nil
			}
		}
	}

	w.done = nil
	w.pty = nil
	w.tty = nil

	return multierror.Append(err, terr, perr).ErrorOrNil()
}

type nullCloser struct{}

func (c *nullCloser) Close() error { return nil }

type pipeWriter struct {
	send func([]byte) error
}

func (w *pipeWriter) Write(p []byte) (int, error) {
	err := w.send(p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func makeStreams(
	sendStdout func([]byte) error, sendStderr func([]byte) error,
	isTerminal bool) (io.Closer, io.Writer, io.Writer, error) {

	stderr := &pipeWriter{send: sendStderr}
	stdout := &pipeWriter{send: sendStdout}

	if isTerminal {
		logging.V(11).Infoln("Opening pseudo terminal")
		pt, tt, err := openPty()
		if err == errUnsupported {
			logging.V(11).Infoln("Pseudo terminal not supported")
			// Fall through, just return plain stdout/err pipes
		} else if err != nil {
			// Fall through, just return plain stdout/err pipes but warn that we tried and failed to make a
			// pty (with coloring because isTerminal means the other side understands ANSI codes)
			stderr.Write([]byte(colors.Always.Colorize(
				colors.SpecWarning + "warning: could not open pty: " + err.Error() + colors.Reset + "\n")))
		} else {
			ptyDone := make(chan error, 1)
			closer := &ptyCloser{
				pty:  pt,
				tty:  tt,
				done: ptyDone,
			}

			go func() {
				_, err = io.Copy(stdout, pt)
				ptyDone <- err
			}()

			// stdout == stderr if we're acting as a terminal
			return closer, tt, tt, nil
		}
	}

	return &nullCloser{}, stdout, stderr, nil
}

// Returns a pair of streams for use with the language runtimes InstallDependencies method
func MakeInstallDependenciesStreams(
	server pulumirpc.LanguageRuntime_InstallDependenciesServer,
	isTerminal bool) (io.Closer, io.Writer, io.Writer, error) {

	return makeStreams(
		func(b []byte) error {
			return server.Send(&pulumirpc.InstallDependenciesResponse{Stdout: b})
		},
		func(b []byte) error {
			return server.Send(&pulumirpc.InstallDependenciesResponse{Stderr: b})
		},
		isTerminal)
}

// Returns a pair of streams for use with the language runtimes RunPlugin method
func MakeRunPluginStreams(
	server pulumirpc.LanguageRuntime_RunPluginServer,
	isTerminal bool) (io.Closer, io.Writer, io.Writer, error) {

	return makeStreams(
		func(b []byte) error {
			return server.Send(&pulumirpc.RunPluginResponse{Output: &pulumirpc.RunPluginResponse_Stdout{Stdout: b}})
		},
		func(b []byte) error {
			return server.Send(&pulumirpc.RunPluginResponse{Output: &pulumirpc.RunPluginResponse_Stderr{Stderr: b}})
		},
		isTerminal)
}
