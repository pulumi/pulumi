package ui

import ui "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/ui"

func PrintTable(table cmdutil.Table, opts *cmdutil.TableRenderOptions) {
	ui.PrintTable(table, opts)
}

func FprintTable(out io.Writer, table cmdutil.Table, opts *cmdutil.TableRenderOptions) {
	ui.FprintTable(out, table, opts)
}

