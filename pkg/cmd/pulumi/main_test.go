// Copyright 2016-2021, Pulumi Corporation.
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

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/pflag"
)

type noTestDeps int

func (noTestDeps) ImportPath() string                          { return "" }
func (noTestDeps) MatchString(pat, str string) (bool, error)   { return false, nil }
func (noTestDeps) SetPanicOnExit0(bool)                        {}
func (noTestDeps) StartCPUProfile(io.Writer) error             { return nil }
func (noTestDeps) StopCPUProfile()                             {}
func (noTestDeps) StartTestLog(io.Writer)                      {}
func (noTestDeps) StopTestLog() error                          { return nil }
func (noTestDeps) WriteProfileTo(string, io.Writer, int) error { return nil }

// flushProfiles flushes test profiles to disk.
func flushProfiles() {
	// Redirect Stdout/err temporarily so the testing code doesn't output the
	// regular:
	//   PASS
	//   coverage: 21.4% of statements
	oldstdout, oldstderr := os.Stdout, os.Stderr
	defer func() {
		os.Stdout, os.Stderr = oldstdout, oldstderr
	}()
	os.Stdout, _ = os.Open(os.DevNull)
	os.Stderr, _ = os.Open(os.DevNull)

	cmdLine := flag.CommandLine
	defer func() { flag.CommandLine = cmdLine }()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	err := flag.CommandLine.Parse(nil)
	contract.IgnoreError(err)

	m := testing.MainStart(noTestDeps(0), nil, nil, nil)
	m.Run()
}

func addGoFlag(pf *pflag.FlagSet, f *flag.Flag) {
	if pf.Lookup(f.Name) != nil || len(f.Name) == 1 && pf.ShorthandLookup(f.Name) != nil {
		return
	}
	pf.AddGoFlag(f)
}

func TestMain(m *testing.M) {
	// If the binary is invoked as `pulumi`, we are being asked to run the coverage-instrumented program. Otherwise,
	// we are running tests as usual.
	if path.Base(os.Args[0]) != "pulumi" {
		flag.Parse()
		os.Exit(m.Run())
	}

	defer panicHandler()

	// Copy the test flags into the Pulumi command's flags.
	cmd := NewPulumiCmd()
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		addGoFlag(cmd.PersistentFlags(), f)
	})

	// Now, execute the Pulumi command and dump coverage data if requested.
	err := cmd.Execute()
	flushProfiles()

	if err != nil {
		_, err = fmt.Fprintf(os.Stderr, "An error occurred: %v\n", err)
		contract.IgnoreError(err)
		os.Exit(1)
	}
}
