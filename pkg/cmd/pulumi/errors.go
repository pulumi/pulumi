// Copyright 2018-2024, Pulumi Corporation.
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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/spf13/cobra"
)

// runCmdFunc wraps cmdutil.RunFunc. While cmdutil.RunFunc provides a standard
// wrapper for dealing with and logging errors before exiting with an
// appropriate error code, runCmdFunc extends this with additional error
// handling specific to the Pulumi CLI. This includes e.g. specific and more
// helpful messages in the case of decryption or snapshot integrity errors.
func runCmdFunc(
	run func(cmd *cobra.Command, args []string) error,
) func(cmd *cobra.Command, args []string) {
	return cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
		err := run(cmd, args)
		return processCmdErrors(err)
	})
}

// Processes errors that may be returned from commands, providing a central
// location to insert more human-friendly messages when certain errors occur, or
// to perform other type-specific handling.
func processCmdErrors(err error) error {
	// If no error occurred, we have nothing to do.
	if err == nil {
		return nil
	}

	// If the error is a "bail" (that is, some expected error flow), then a
	// diagnostic or message will already have been reported. We can thus return
	// it in order to effect the exit code without printing the error message
	// again.
	if result.IsBail(err) {
		return err
	}

	// Other type-specific error handling.
	if de, ok := engine.AsDecryptError(err); ok {
		printDecryptError(*de)
		return nil
	} else if sie, ok := deploy.AsSnapshotIntegrityError(err); ok {
		printSnapshotIntegrityError(err, *sie)

		// Having printed out a specific error, we don't want RunFunc to print out
		// the underlying message again. We do however want it to exit with a
		// non-zero exit code, so we return a BailError to satisfy both these needs.
		return result.BailError(err)
	}

	// In all other cases, return the unexpected error as-is for generic handling.
	return err
}

// A type-specific handler for engine.DecryptErrors that prints out help text
// containing common causes of and possible resolutions for decryption errors.
func printDecryptError(e engine.DecryptError) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	fprintf(writer, "failed to decrypt encrypted configuration value '%s': %s\n", e.Key, e.Err)
	fprintf(writer, ""+
		"This can occur when a secret is copied from one stack to another. Encryption of secrets is done per-stack and "+
		"it is not possible to share an encrypted configuration value across stacks.\n"+
		"\n"+
		"You can re-encrypt your configuration by running `pulumi config set %s [value] --secret` with your "+
		"new stack selected.\n"+
		"\n"+
		"refusing to proceed", e.Key)
	contract.IgnoreError(writer.Flush())
	cmdutil.Diag().Errorf(diag.RawMessage("" /*urn*/, buf.String()))
}

// Snapshot integrity errors are generally indicative of a serious bug in the
// Pulumi engine. This function is a type-specific handler for these errors that
// prints out a panic-like banner with information about the error and how to
// report it.
func printSnapshotIntegrityError(err error, sie deploy.SnapshotIntegrityError) {
	readOrWrite := ""
	if sie.Op == deploy.SnapshotIntegrityRead {
		readOrWrite = `
NOTE: This error occurred while reading a snaphot. This error was introduced by
a previous operation when it wrote the snapshot. If you have details about that
operation, please include them in your report as well.
`
	}

	cmdutil.Diag().Errorf(diag.RawMessage(
		"", /*urn*/
		fmt.Sprintf(`The Pulumi CLI encountered a snapshot integrity error. This is a bug!

================================================================================
We would appreciate a report: https://github.com/pulumi/pulumi/issues/
%s
Please provide all of the text below in your report.
================================================================================
Pulumi Version:    %s
Go Version:        %s
Go Compiler:       %s
Architecture:      %s
Operating System:  %s
Command:           %s
Error:             %s

Stack Trace:

%s
`,
			readOrWrite,
			version.Version,
			runtime.Version(),
			runtime.Compiler,
			runtime.GOARCH,
			runtime.GOOS,
			strings.Join(os.Args, " "),
			err,
			string(sie.Stack),
		),
	))
}

// Quick and dirty utility function for printing to writers that we know will never fail.
func fprintf(writer io.Writer, msg string, args ...interface{}) {
	_, err := fmt.Fprintf(writer, msg, args...)
	contract.IgnoreError(err)
}
