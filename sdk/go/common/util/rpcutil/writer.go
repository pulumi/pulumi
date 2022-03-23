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
	"os"

	"github.com/creack/pty"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type stdoutWriter struct {
	server   pulumirpc.LanguageRuntime_InstallDependenciesServer
	done     chan (bool)
	pty, tty *os.File
}

func (w *stdoutWriter) Write(p []byte) (int, error) {
	data := pulumirpc.InstallDependenciesResponse{}
	data.Stdout = p

	err := w.server.Send(&data)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (w *stdoutWriter) Close() error {
	// Try to close the pty and tty if we have them
	if w.tty != nil {
		contract.Assert(w.pty != nil)
		terr := w.tty.Close()
		perr := w.pty.Close()

		if terr != nil {
			return terr
		}
		if perr != nil {
			return perr
		}
	}

	// Wait for the done signal if we have one
	if w.done != nil {
		<-w.done
	}
	return nil
}

type stderrWriter struct {
	server pulumirpc.LanguageRuntime_InstallDependenciesServer
}

func (w *stderrWriter) Write(p []byte) (int, error) {
	data := pulumirpc.InstallDependenciesResponse{}
	data.Stderr = p

	err := w.server.Send(&data)
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

// Returns a pair of streams for use with the language runtimes InstallDependencies method
func MakeStreams(server pulumirpc.LanguageRuntime_InstallDependenciesServer, isTerminal bool) (io.WriteCloser, io.Writer, error) {
	if isTerminal {
		pt, tt, err := pty.Open()
		if err == pty.ErrUnsupported {
			// Fall through, just return plain stdout/err pipes
		} else if err != nil {
			return nil, nil, err
		} else {
			ptyDone := make(chan bool, 1)
			stdout := &stdoutWriter{
				server: server,
				pty:    pt,
				tty:    tt,
				done:   ptyDone,
			}

			go func() {
				_, _ = io.Copy(stdout, pt)
				ptyDone <- true
			}()

			// stdout == stderr if we're acting as a terminal
			return stdout, stdout, nil
		}
	}

	stdout := &stdoutWriter{
		server: server,
	}

	stderr := &stderrWriter{
		server: server,
	}

	return stdout, stderr, nil
}
