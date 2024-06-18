package main

import (
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type Options struct {
	// Options to control backend behavior.
	Backend BackendOptions
	// Options to control displayed output.
	Display DisplayOptions
}

// BackendOptions represents the set of options that can be configured by the
// user to control how Pulumi backends (e.g. HTTP or local storage) behave
// during an execution of the Pulumi CLI.
type BackendOptions struct {
	// True if state checkpoint saving should be skipped and only the final
	// deployment state should be saved.
	SkipCheckpoints bool
}

// DisplayOptions represents the set of display options that can be configured
// by the user for use during an execution of the Pulumi CLI.
type DisplayOptions struct {
	// Colorization to apply to events.
	Color colors.Colorization
	// True if we should show configuration information.
	ShowConfig bool
	// True if we should show rich diffs detailing changes.
	ShowDiff bool
	// True if we should show detailed policy remediations.
	ShowPolicyRemediations bool
	// True to show the replacement steps in the plan.
	ShowReplacementSteps bool
	// True to show the resources that aren't updated in addition to updates.
	ShowSameResources bool
	// True to show resources that are being read in
	ShowReads bool
	// True if we should truncate long outputs
	TruncateOutput bool
	// True to suppress output summarization, e.g. if contains sensitive info.
	SuppressOutputs bool
	// The string "true" if permalinks should be suppressed. All other strings,
	// including the empty string (the default), will be interpreted as false.
	SuppressPermalink string
	// True if we should display things interactively.
	IsInteractive bool
	// True if we should emit the entire diff as JSON.
	JSONDisplay bool
	// The path to the file to use for logging events, if any.
	EventLogPath string
	// True to enable debug output.
	Debug bool
	// True to suppress displaying progress spinner.
	SuppressProgress bool
}

type OptionsBuilder struct {
	name string
	opts *Options
	cmd  *cobra.Command
}

func NewOptionsBuilder(name string, cmd *cobra.Command) *OptionsBuilder {
	return &OptionsBuilder{name: name, cmd: cmd, opts: &Options{}}
}

func (b *OptionsBuilder) WithDisplayColor() *OptionsBuilder {
	b.opts.Display.Color = cmdutil.GetGlobalColorization()
	return b
}

func (b *OptionsBuilder) WithDisplayDebug() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.Debug, "debug", false,
		"Print detailed debugging output")

	return b
}

func (b *OptionsBuilder) WithDisplayEventLogPath() *OptionsBuilder {
	b.cmd.Flags().StringVar(
		&b.opts.Display.EventLogPath, "event-log", "",
		"Log events to a file at this path")

	return b
}

func (b *OptionsBuilder) WithDisplayIsInteractive() *OptionsBuilder {
	b.opts.Display.IsInteractive = cmdutil.Interactive()
	return b
}

func (b *OptionsBuilder) WithDisplayJSON() *OptionsBuilder {
	b.cmd.Flags().BoolVarP(
		&b.opts.Display.JSONDisplay, "json", "j", false,
		"Emit output as JSON")

	return b
}

func (b *OptionsBuilder) WithDisplayShowConfig() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowConfig, "show-config", false,
		"Show configuration keys and variables")

	return b
}

func (b *OptionsBuilder) WithDisplayShowDiff() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowDiff, "show-diff", false,
		"Display operation as a rich diff showing the overall change")

	return b
}

func (b *OptionsBuilder) WithDisplayShowPolicyRemediations() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowPolicyRemediations, "show-policy-remediations", false,
		"Show per-resource policy remediation details instead of a summary")

	return b
}

func (b *OptionsBuilder) WithDisplayShowReads() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")

	return b
}

func (b *OptionsBuilder) WithDisplayShowReplacementSteps() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")

	return b
}

func (b *OptionsBuilder) WithDisplayShowSameResources() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.ShowSameResources, "show-sames", false,
		"Show resources that don't need to be updated because they haven't changed, alongside those that do")

	return b
}

func (b *OptionsBuilder) WithDisplaySuppressOutputs() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.SuppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")

	return b
}

func (b *OptionsBuilder) WithDisplaySuppressProgress() *OptionsBuilder {
	b.cmd.Flags().BoolVar(
		&b.opts.Display.SuppressProgress, "suppress-progress", false,
		"Suppress display of periodic progress dots")

	return b
}

func (b *OptionsBuilder) WithDisplaySuppressPermalink() *OptionsBuilder {
	b.cmd.Flags().StringVar(
		&b.opts.Display.SuppressPermalink, "suppress-permalink", "",
		"Suppress display of the state permalink")

	b.cmd.Flag("suppress-permalink").NoOptDefVal = "false"
	return b
}

func (b *OptionsBuilder) Build() *Options {
	return b.opts
}

func (o Options) AsDisplayOptions() display.Options {
	displayOpts := display.Options{
		Color:                  o.Display.Color,
		ShowConfig:             o.Display.ShowConfig,
		ShowPolicyRemediations: o.Display.ShowPolicyRemediations,
		ShowReplacementSteps:   o.Display.ShowReplacementSteps,
		ShowSameResources:      o.Display.ShowSameResources,
		ShowReads:              o.Display.ShowReads,
		TruncateOutput:         o.Display.TruncateOutput,
		SuppressOutputs:        o.Display.SuppressOutputs,
		IsInteractive:          o.Display.IsInteractive,
		JSONDisplay:            o.Display.JSONDisplay,
		EventLogPath:           o.Display.EventLogPath,
		Debug:                  o.Display.Debug,
		SuppressProgress:       o.Display.SuppressProgress,
	}

	displayOpts.Type = display.DisplayProgress
	if o.Display.ShowDiff {
		displayOpts.Type = display.DisplayDiff
	}

	if o.Display.SuppressPermalink == "true" {
		displayOpts.SuppressPermalink = true
	} else {
		displayOpts.SuppressPermalink = false
	}

	return displayOpts
}
