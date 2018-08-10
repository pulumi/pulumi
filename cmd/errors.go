package cmd

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/pulumi/pulumi/pkg/diag"
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
	default:
		return err
	}
}

func printPendingOperationsError(e deploy.PlanPendingOperationsError) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	fmt.Fprintf(writer,
		"the current deployment has %d resource(s) with pending operations:\n", len(e.Operations))

	for _, op := range e.Operations {
		fmt.Fprintf(writer, "  * %s, interrupted while %s\n", op.Resource.URN, op.Operation)
	}

	fmt.Fprintf(writer, "\n")
	fmt.Fprintf(writer, "These resources are in an unknown state because the Pulumi CLI was interrupted while\n")
	fmt.Fprintf(writer, "waiting for changes to these resources to complete. You should confirm whether or not the\n")
	fmt.Fprintf(writer, "operations listed completed successfully by checking the state of the appropriate provider.\n")
	fmt.Fprintf(writer, "For example, if you are using AWS, you can confirm using the AWS Console.\n")
	fmt.Fprintf(writer, "\n")
	fmt.Fprintf(writer, "Once you have confirmed the status of the interrupted operations, you can repair your stack\n")
	fmt.Fprintf(writer, "using `pulumi stack export` to export your stack to a file. For each operation that succeeded,\n")
	fmt.Fprintf(writer, "remove the `status` field. Once this is complete, use `pulumi stack import` to import the\n")
	fmt.Fprintf(writer, "repaired stack.")
	contract.IgnoreError(writer.Flush())

	cmdutil.Diag().Errorf(diag.RawMessage("" /*urn*/, buf.String()))
}
