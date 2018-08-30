package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// PrintEngineError optionally provides a place for the CLI to provide human-friendly error
// messages for messages that can happen during normal engine operation.
func PrintEngineError(err error) error {
	if err == nil {
		return nil
	}

	switch e := err.(type) {
	case deploy.PlanPendingOperationsError:
		printPendingOperationsError(e)
		return fmt.Errorf("refusing to proceed")
	case engine.DecryptError:
		printDecryptError(e)
		return fmt.Errorf("refusing to proceed")
	default:
		return err
	}
}

func printPendingOperationsError(e deploy.PlanPendingOperationsError) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	fprintf(writer,
		"the current deployment has %d resource(s) with pending operations:\n", len(e.Operations))

	for _, op := range e.Operations {
		fprintf(writer, "  * %s, interrupted while %s\n", op.Resource.URN, op.Type)
	}

	fprintf(writer, `
These resources are in an unknown state because the Pulumi CLI was interrupted while
waiting for changes to these resources to complete. You should confirm whether or not the
operations listed completed successfully by checking the state of the appropriate provider.
For example, if you are using AWS, you can confirm using the AWS Console.

Once you have confirmed the status of the interrupted operations, you can repair your stack
using 'pulumi stack export' to export your stack to a file. For each operation that succeeded,
remove that operation from the "pending_operations" section of the file. Once this is complete,
use 'pulumi stack import' to import the repaired stack.`)
	contract.IgnoreError(writer.Flush())

	cmdutil.Diag().Errorf(diag.RawMessage("" /*urn*/, buf.String()), false)
}

func printDecryptError(e engine.DecryptError) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	fprintf(writer, "failed to decrypt encrypted configuration value '%s': %s", e.Key, e.Err.Error())
	fprintf(writer, `
This can occur when a secret is copied from one stack to another. Encryption of secrets is done per-stack and
it is not possible to share an encrypted configuration value across stacks.

You can re-encrypt your configuration buy running 'pulumi config set %s [value] --secret' with your
new stack selected.`, e.Key)
	contract.IgnoreError(writer.Flush())
	cmdutil.Diag().Errorf(diag.RawMessage("" /*urn*/, buf.String()), false)
}

// Quick and dirty utility function for printing to writers that we know will never fail.
func fprintf(writer io.Writer, msg string, args ...interface{}) {
	_, err := fmt.Fprintf(writer, msg, args...)
	contract.IgnoreError(err)
}
