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
	"io/ioutil"
	"os"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
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

	closer, stdout, stderr, err := MakeStreams(server, false)
	assert.NoError(t, err)

	// stdout and stderr should just write to server
	l, err := stdout.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, l)

	l, err = stderr.Write([]byte("world"))
	assert.NoError(t, err)
	assert.Equal(t, 5, l)

	err = closer.Close()
	assert.NoError(t, err)

	outBytes, err := ioutil.ReadAll(&server.stdout)
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), outBytes)

	errBytes, err := ioutil.ReadAll(&server.stderr)
	assert.NoError(t, err)
	assert.Equal(t, []byte("world"), errBytes)
}

func TestWriter_Terminal(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, stdout, stderr, err := MakeStreams(server, true)
	assert.NoError(t, err)

	// We _may_ have made a pty and stdout and stderr are the same and both send to the server as stdout
	if stdout == stderr {
		l, err := stdout.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.Equal(t, 5, l)

		l, err = stderr.Write([]byte("world"))
		assert.NoError(t, err)
		assert.Equal(t, 5, l)

		err = closer.Close()
		assert.NoError(t, err)

		outBytes, err := ioutil.ReadAll(&server.stdout)
		assert.NoError(t, err)
		assert.Equal(t, []byte("helloworld"), outBytes)

		errBytes, err := ioutil.ReadAll(&server.stderr)
		assert.NoError(t, err)
		assert.Equal(t, []byte{}, errBytes)
	} else {
		// else they are separate and should behave just like the NoTerminal case
		l, err := stdout.Write([]byte("hello"))
		assert.NoError(t, err)
		assert.Equal(t, 5, l)

		l, err = stderr.Write([]byte("world"))
		assert.NoError(t, err)
		assert.Equal(t, 5, l)

		err = closer.Close()
		assert.NoError(t, err)

		outBytes, err := ioutil.ReadAll(&server.stdout)
		assert.NoError(t, err)
		assert.Equal(t, []byte("hello"), outBytes)

		errBytes, err := ioutil.ReadAll(&server.stderr)
		assert.NoError(t, err)
		assert.Equal(t, []byte("world"), errBytes)
	}
}

func TestWriter_IsPTY(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, stdout, stderr, err := MakeStreams(server, true)
	assert.NoError(t, err)

	// We _may_ have made a pty, check IsTerminal returns true
	if stdout == stderr {
		// These will be os.Files if a pty
		file, ok := stdout.(*os.File)
		assert.True(t, ok, "stdout was not a File")
		assert.True(t, term.IsTerminal(int(file.Fd())), "stdout was not a terminal")
	}

	err = closer.Close()
	assert.NoError(t, err)
}

func TestWriter_SafeToCloseTwice(t *testing.T) {
	t.Parallel()

	server := makeStreamMock()

	closer, _, _, err := MakeStreams(server, true)
	assert.NoError(t, err)

	err = closer.Close()
	assert.NoError(t, err)

	err = closer.Close()
	assert.NoError(t, err)
}
