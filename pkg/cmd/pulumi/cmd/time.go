package cmd

import cmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/cmd"

// FormatTime formats the given time.Time according to RFC 5424, with millisecond precision.
func FormatTime(t time.Time) string {
	return cmd.FormatTime(t)
}

