package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// PrintEngineResult optionally provides a place for the CLI to provide human-friendly error
// messages for messages that can happen during normal engine operation.
func PrintEngineResult(res result.Result) result.Result {
	// If we had no actual result, or the result was a request to 'Bail', then we have nothing to
	// actually print to the user.
	if res == nil || res.IsBail() {
		return res
	}

	err := res.Error()

	switch e := err.(type) {
	case engine.DecryptError:
		printDecryptError(e)
		// We have printed the error already.  Should just bail at this point.
		return result.Bail()
	default:
		// Caller will handle printing of this true error in a generalized fashion.
		return res
	}
}

func printDecryptError(e engine.DecryptError) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	fprintf(writer, "failed to decrypt encrypted configuration value '%s': %s", e.Key, e.Err)
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

// Quick and dirty utility function for printing to writers that we know will never fail.
func fprintf(writer io.Writer, msg string, args ...interface{}) {
	_, err := fmt.Fprintf(writer, msg, args...)
	contract.IgnoreError(err)
}
