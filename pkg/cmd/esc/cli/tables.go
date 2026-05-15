// Copyright 2026, Pulumi Corporation.

package cli

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// newTable returns a table.Writer configured with the esc CLI's standard style.
// Callers are expected to append a header, append rows, and then Render().
func newTable(stdout io.Writer) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(stdout)
	t.SetStyle(table.StyleLight)
	return t
}
