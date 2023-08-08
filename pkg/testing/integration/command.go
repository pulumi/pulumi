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

package integration

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// RunCommand executes the specified command and additional arguments, wrapping any output in the
// specialized test output streams that list the location the test is running in.
func RunCommand(t testing.TB, name string, args []string, wd string, opts *ProgramTestOptions) (err error) {
	if opts == nil {
		opts = &ProgramTestOptions{}
	}

	path := args[0]
	command := strings.Join(args, " ")
	t.Logf("**** Invoke '%v' in '%v'", command, wd)

	env := append(os.Environ(), opts.Env...)
	env = append(env,
		"PULUMI_DEBUG_COMMANDS=true",
		"PULUMI_RETAIN_CHECKPOINTS=true",
		"PULUMI_CONFIG_PASSPHRASE=correct horse battery staple")

	cmd := exec.Cmd{
		Path: path,
		Dir:  wd,
		Args: args,
		Env:  env,
	}

	var logFile io.Writer
	{
		f, err := openLogFile(name, wd)
		if err != nil {
			return fmt.Errorf("opening log file: %v", err)
		}
		t.Logf("**** Logging to %v", f.Name())
		defer func() {
			if err := f.Close(); err != nil {
				t.Errorf("Error closing log file: %v", err)
			}
		}()

		// os.File is not safe for concurrent writes, so we need to guard it
		// because we may be writing to it from multiple goroutines.
		logFile = &lockedWriter{W: f}
	}

	// Note that testWriter is safe for concurrent use.
	testWriter := iotest.LogWriterPrefixed(t, fmt.Sprintf("[%v] ", name))

	// We want to make sure no command output is lost.
	// Output *always* goes to the log file.
	// On top of that, we have two modes:
	//
	// 1. Verbose mode:
	//    output goes to stdout/stderr if specified,
	//    or the test log if not.
	// 2. Non-verbose mode:
	//    output goes to the test log only by default.
	//    If stdout/stderr are specified *and* the command failed,
	//    output is also written to them.
	if opts.Verbose || os.Getenv("PULUMI_VERBOSE_TEST") != "" {
		stdout := opts.Stdout
		if stdout == nil {
			stdout = testWriter
		}

		stderr := opts.Stderr
		if stderr == nil {
			stderr = testWriter
		}

		cmd.Stdout = io.MultiWriter(logFile, stdout)
		cmd.Stderr = io.MultiWriter(logFile, stderr)
	} else {
		// Stdout and stderr always go to log file and the test writer.
		w := io.MultiWriter(logFile, testWriter)
		stdout, stderr := w, w

		// If opts.Stdout or opts.Stderr are set,
		// also buffer that stream and flush to that writer
		// if the command fails.
		if opts.Stdout != nil {
			var buf bytes.Buffer
			stdout = io.MultiWriter(stdout, &buf)
			defer func() {
				if err == nil {
					return
				}

				if _, werr := opts.Stdout.Write(buf.Bytes()); werr != nil {
					t.Errorf("Error writing stdout: %v", werr)
				}
			}()
		}
		if opts.Stderr != nil {
			var buf bytes.Buffer
			stderr = io.MultiWriter(stderr, &buf)
			defer func() {
				if err == nil {
					return
				}

				if _, werr := opts.Stderr.Write(buf.Bytes()); werr != nil {
					t.Errorf("Error writing stderr: %v", werr)
				}
			}()
		}

		cmd.Stdout = stdout
		cmd.Stderr = stderr
		// Same stdout/stderr, no synchronization needed.
	}

	startTime := time.Now()
	runerr := cmd.Run()
	endTime := time.Now()

	if opts.ReportStats != nil {
		// Note: This data is archived and used by external analytics tools.  Take care if changing the schema or format
		// of this data.
		opts.ReportStats.ReportCommand(TestCommandStats{
			StartTime:      startTime.Format("2006/01/02 15:04:05"),
			EndTime:        endTime.Format("2006/01/02 15:04:05"),
			ElapsedSeconds: float64((endTime.Sub(startTime)).Nanoseconds()) / 1000000000,
			StepName:       name,
			CommandLine:    command,
			StackName:      string(opts.GetStackName()),
			TestID:         wd,
			TestName:       filepath.Base(opts.Dir),
			IsError:        runerr != nil,
			CloudURL:       opts.CloudURL,
		})
	}

	if runerr != nil {
		t.Logf("Invoke '%v' failed: %s\n", command, cmdutil.DetailedError(runerr))
	}

	return runerr
}

func withOptionalYarnFlags(args []string) []string {
	flags := os.Getenv("YARNFLAGS")

	if flags != "" {
		return append(args, flags)
	}

	return args
}

// addFlagIfNonNil will take a set of command-line flags, and add a new one if the provided flag value is not empty.
func addFlagIfNonNil(args []string, flag, flagValue string) []string {
	if flagValue != "" {
		args = append(args, flag, flagValue)
	}
	return args
}

// lockedWriter adds thread-safety to any writer
// by serializing all writes to it.
type lockedWriter struct {
	W io.Writer // underlying writer

	mu sync.Mutex
}

func (lw *lockedWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.W.Write(p)
}
