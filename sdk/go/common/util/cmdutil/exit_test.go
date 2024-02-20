// Copyright 2023-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdutil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunFunc_Bail(t *testing.T) {
	t.Parallel()

	// Verifies that a use of RunFunc that returns BailError
	// will cause the program to exit with a non-zero exit code
	// without printing an error message.
	//
	// Unfortunately, we can't test this directly,
	// because the `os.Exit` call in RunResultFunc.
	//
	// Instead, we'll re-run the test binary,
	// and have it run TestFakeCommand.
	// We'll verify the output of that binary instead.

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command(exe, "-test.run=^TestFakeCommand$")
	cmd.Env = append(os.Environ(), "TEST_FAKE=1")

	// Write output to the buffer and to the test logger.
	var buff bytes.Buffer
	output := io.MultiWriter(&buff, iotest.LogWriter(t))
	cmd.Stdout = output
	cmd.Stderr = output

	err = cmd.Run()
	exitErr := new(exec.ExitError)
	require.ErrorAs(t, err, &exitErr)
	assert.NotZero(t, exitErr.ExitCode())

	assert.Empty(t, buff.String())
}

//nolint:paralleltest // not a real test
func TestFakeCommand(t *testing.T) {
	if os.Getenv("TEST_FAKE") != "1" {
		// This is not a real test.
		// It's a fake test that we'll run as a subprocess
		// to verify that the RunFunc function works correctly.
		// See TestRunFunc_Bail for more details.
		return
	}

	cmd := &cobra.Command{
		Run: RunFunc(func(cmd *cobra.Command, args []string) error {
			return result.BailErrorf("bail")
		}),
	}
	err := cmd.Execute()
	// Unreachable: RunFunc should have called os.Exit.
	assert.Fail(t, "unreachable", "RunFunc should have called os.Exit: %v", err)
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give error
		want string
	}{
		{
			desc: "simple error",
			give: errors.New("great sadness"),
			want: "great sadness",
		},
		{
			desc: "hashi multi error",
			give: multierror.Append(
				errors.New("foo"),
				errors.New("bar"),
				errors.New("baz"),
			),
			want: "3 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz",
		},
		{
			desc: "std errors.Join",
			give: errors.Join(
				errors.New("foo"),
				errors.New("bar"),
				errors.New("baz"),
			),
			want: "3 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz",
		},
		{
			desc: "empty multi error",
			// This is technically invalid,
			// but we guard against it,
			// so let's test it too.
			give: &invalidEmptyMultiError{},
			want: "invalid empty multi error",
		},
		{
			desc: "single wrapped error",
			give: &multierror.Error{
				Errors: []error{
					errors.New("great sadness"),
				},
			},
			want: "great sadness",
		},
		{
			desc: "error trees (left-nested)",
			give: errors.Join(
				errors.Join(
					errors.New("foo"),
					errors.New("bar"),
				),
				errors.New("baz"),
			),
			want: "3 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz",
		},
		{
			desc: "error trees (right-nested)",
			give: errors.Join(
				errors.New("foo"),
				errors.Join(
					errors.New("bar"),
					errors.New("baz"),
				),
			),
			want: "3 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz",
		},
		{
			desc: "error trees (mixed)",
			give: errors.Join(
				errors.Join(
					errors.New("foo"),
					errors.Join(
						errors.New("bar"),
						errors.New("baz"),
					),
				),
				errors.Join(
					errors.Join(
						errors.New("quux"),
						errors.New("frob"),
					),
					errors.New("urk"),
					errors.New("blog"),
				),
			),
			want: "7 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz" +
				"\n    4) quux" +
				"\n    5) frob" +
				"\n    6) urk" +
				"\n    7) blog",
		},
		{
			desc: "multi error inside single wrapped error",
			give: &multierror.Error{
				Errors: []error{
					errors.Join(
						errors.New("foo"),
						errors.New("bar"),
						errors.New("baz"),
					),
				},
			},
			want: "3 errors occurred:" +
				"\n    1) foo" +
				"\n    2) bar" +
				"\n    3) baz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got := errorMessage(tt.give)
			assert.Equal(t, tt.want, got)
		})
	}
}

// invalidEmptyMultiError is an invalid error type
// that implements Unwrap() []error, but returns an empty slice.
// This is invalid per the contract for that method.
type invalidEmptyMultiError struct{}

func (*invalidEmptyMultiError) Error() string {
	return "invalid empty multi error"
}

func (*invalidEmptyMultiError) Unwrap() []error {
	return []error{} // invalid
}
