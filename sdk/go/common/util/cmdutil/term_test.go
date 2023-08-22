// Copyright 2016-2018, Pulumi Corporation.
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

package cmdutil

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if os.Getenv("INSIDE_TEST") == "" {
		os.Exit(m.Run())
	}

	sigch := make(chan os.Signal, 2)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Waiting for SIGINT...")
	select {
	case <-sigch:
		fmt.Println("SIGINT received, cleaning up...")
		time.Sleep(1 * time.Second)

	case <-time.After(3 * time.Second):
		fmt.Println("Timed out waiting for SIGINT, exiting...")
		os.Exit(1)
	}
}

func TestTerminate(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)

	// Don't run tests, just the TestMain.
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = []string{"INSIDE_TEST=1"}
	RegisterProcessGroup(cmd)
	cmd.Stdout = iotest.LogWriterPrefixed(t, "child(stdout): ")
	cmd.Stderr = iotest.LogWriterPrefixed(t, "child(stderr): ")

	t.Log("Starting child process")
	require.NoError(t, cmd.Start())

	go func() {
		time.Sleep(1 * time.Second)
		t.Log("Sending SIGINT")
		assert.NoError(t, TerminateProcess(cmd.Process, 5*time.Second))
	}()

	t.Log("Waiting for child process to exit")
	require.NoError(t, cmd.Wait())

	t.Log("Child process exited")
}
