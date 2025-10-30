package client

import client "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate/client"

// TruncateWithMiddleOut takes a string and a maximum character count, and returns a new string with content truncated
// from the middle if the total character count exceeds maxChars. This preserves both the beginning and end of the
// content while removing content from the middle.
func TruncateWithMiddleOut(content string, maxChars int) string {
	return client.TruncateWithMiddleOut(content, maxChars)
}

