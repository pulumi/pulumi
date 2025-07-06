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
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"golang.org/x/term"
)

func makeStreamMock() *streamMock {
	return &streamMock{
		ctx: context.Background(),
	}
}

type streamMock struct {
	grpc.ServerStream
	ctx    context.Context
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func (m *streamMock) Context() context.Context {
	return m.ctx
}

func (m *streamMock) Send(resp *pulumirpc.InstallDependenciesResponse) error {
	if _, err := m.stdout.Write(resp.Stdout); err != nil {
		return err
	}
	if _, err := m.stderr.Write(resp.Stderr); err != nil {
		return err
	}
	return nil
}

func TestWriter_NoTerminal(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, stdout, stderr, err := MakeInstallDependenciesStreams(server, false)
	require.NoError(t, err)

	// stdout and stderr should just write to server
	l, err := stdout.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, l)

	l, err = stderr.Write([]byte("world"))
	require.NoError(t, err)
	assert.Equal(t, 5, l)

	err = closer.Close()
	require.NoError(t, err)

	outBytes, err := io.ReadAll(&server.stdout)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), outBytes)

	errBytes, err := io.ReadAll(&server.stderr)
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), errBytes)
}

func TestWriter_Terminal(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, stdout, stderr, err := MakeInstallDependenciesStreams(server, true)
	require.NoError(t, err)

	// We _may_ have made a pty and stdout and stderr are the same and both send to the server as stdout
	if stdout == stderr {
		// osx behaves strangely reading and writing a pty in the same process, and we want to check that this
		// is still a tty when invoking other processes so invoke the "test" program.

		// We can't use the tty program because that tests stdin and we aren't setting stdin, but "test" can
		// check if file descriptor 1 (i.e. stdout) is a tty with -t.
		cmd := exec.Command("test", "-t", "1")
		cmd.Stdin = nil
		cmd.Stdout = stdout
		cmd.Stderr = stdout

		err := cmd.Run()
		require.NoError(t, err)

		exitcode := cmd.ProcessState.ExitCode()
		assert.Equal(t, 0, exitcode)

		// Now check we can reuse the stream to echo some text back
		text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit," +
			"sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.\n" +
			"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.\n" +
			"Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.\n" +
			"Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.\n"
		cmd = exec.Command("echo", text)
		cmd.Stdin = nil
		cmd.Stdout = stdout
		cmd.Stderr = stdout

		err = cmd.Run()
		require.NoError(t, err)

		exitcode = cmd.ProcessState.ExitCode()
		assert.Equal(t, 0, exitcode)

		err = closer.Close()
		require.NoError(t, err)

		outBytes, err := io.ReadAll(&server.stdout)
		require.NoError(t, err)
		// echo adds an extra \n at the end, and line discipline will cause \n to come back as \r\n
		expected := strings.ReplaceAll(text+"\n", "\n", "\r\n")
		assert.Equal(t, []byte(expected), outBytes)

		errBytes, err := io.ReadAll(&server.stderr)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, errBytes)
	} else {
		// else they are separate and should behave just like the NoTerminal case
		l, err := stdout.Write([]byte("hello"))
		require.NoError(t, err)
		assert.Equal(t, 5, l)

		l, err = stderr.Write([]byte("world"))
		require.NoError(t, err)
		assert.Equal(t, 5, l)

		err = closer.Close()
		require.NoError(t, err)

		outBytes, err := io.ReadAll(&server.stdout)
		require.NoError(t, err)
		assert.Equal(t, []byte("hello"), outBytes)

		errBytes, err := io.ReadAll(&server.stderr)
		require.NoError(t, err)
		assert.Equal(t, []byte("world"), errBytes)
	}
}

func TestWriter_IsPTY(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, stdout, stderr, err := MakeInstallDependenciesStreams(server, true)
	require.NoError(t, err)

	// We _may_ have made a pty, check IsTerminal returns true
	if stdout == stderr {
		// These will be os.Files if a pty
		file, ok := stdout.(*os.File)
		assert.True(t, ok, "stdout was not a File")
		//nolint:gosec
		assert.True(t, term.IsTerminal(int(file.Fd())), "stdout was not a terminal")
	}

	err = closer.Close()
	require.NoError(t, err)
}

func TestWriter_SafeToCloseTwice(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, _, _, err := MakeInstallDependenciesStreams(server, true)
	require.NoError(t, err)

	err = closer.Close()
	require.NoError(t, err)

	err = closer.Close()
	require.NoError(t, err)
}
