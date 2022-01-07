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

package cmdutil

import (
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKillChildren(t *testing.T) {
	d := t.TempDir()

	run := func(kill bool, logFile string) {
		cmd := exec.Command("go", "run", "processtree.go",
			"-depth", "3",
			"-out", logFile)

		cmd.Dir = "testdata"

		if kill {
			RegisterProcessGroup(cmd)
		}

		err := cmd.Start()
		require.NoError(t, err)

		if kill {
			err := KillChildren(cmd.Process.Pid)
			require.NoError(t, err)
		} else {
			err = cmd.Wait()
			require.NoError(t, err)
			require.Equal(t, 0, cmd.ProcessState.ExitCode())
		}
	}

	readFile := func(path string) (string, error) {
		attempt := 1
		delay := 50 * time.Millisecond
		maxAttempts := 5
		for {
			bytes, err := ioutil.ReadFile(path)
			if err != nil || len(bytes) == 0 {
				if attempt == maxAttempts {
					return "", err
				}
				attempt = attempt + 1
				time.Sleep(delay)
				delay = 2 * delay
				continue
			}

			return string(bytes), nil
		}
	}

	// First check the test process writes OK to ok.log via a
	// descendant process when allowed to run normally.
	okLog := filepath.Join(d, "ok.log")
	run(false, okLog)
	contents, err := readFile(okLog)
	require.NoError(t, err)
	require.Equal(t, "OK", contents)

	// Now check that nothing gets written if we do kill the
	// group. Indirectly this verifies the descendant has been
	// killed.
	noLog := filepath.Join(d, "no.log")
	run(true, noLog)
	_, err = readFile(noLog)
	require.Error(t, err)
}
