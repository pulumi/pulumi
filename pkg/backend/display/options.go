// Copyright 2016-2018, Pulumi Corporation.
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

package display

import (
	"io"

	"github.com/pulumi/pulumi/pkg/v3/backend/display/internal/terminal"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// Type of output to display.
type Type int

const (
	// DisplayProgress displays an update as it progresses.
	DisplayProgress Type = iota
	// DisplayDiff displays a rich diff.
	DisplayDiff
	// DisplayQuery displays query output.
	DisplayQuery
	// DisplayWatch displays watch output.
	DisplayWatch
)

// Options controls how the output of events are rendered
type Options struct {
	Color                  colors.Colorization // colorization to apply to events.
	ShowConfig             bool                // true if we should show configuration information.
	ShowPolicyRemediations bool                // true if we should show detailed policy remediations.
	ShowResourceChanges    bool                // true if we should print detailed resource changes.
	ShowReplacementSteps   bool                // true to show the replacement steps in the plan.
	ShowSameResources      bool                // true to show the resources that aren't updated in addition to updates.
	ShowReads              bool                // true to show resources that are being read in
	TruncateOutput         bool                // true if we should truncate long outputs
	SuppressOutputs        bool                // true to suppress output summarization, e.g. if contains sensitive info.
	SuppressPermalink      bool                // true to suppress state permalink (including in DIY backends)
	SummaryDiff            bool                // true if diff display should be summarized.
	IsInteractive          bool                // true if we should display things interactively.
	Type                   Type                // type of display (rich diff, progress, or query).
	JSONDisplay            bool                // true if we should emit the entire diff as JSON.
	EventLogPath           string              // the path to the file to use for logging events, if any.
	Debug                  bool                // true to enable debug output.
	Stdin                  io.Reader           // the reader to use for stdin. Defaults to os.Stdin if unset.
	Stdout                 io.Writer           // the writer to use for stdout. Defaults to os.Stdout if unset.
	Stderr                 io.Writer           // the writer to use for stderr. Defaults to os.Stderr if unset.
	SuppressTimings        bool                // true to suppress displaying timings of resource actions
	SuppressProgress       bool                // true to suppress displaying progress spinner.
	ShowLinkToCopilot      bool                // true to display a 'explainFailure' link to Copilot.
	ShowSecrets            bool                // true to display secrets in the output.
	// Low level options
	term                terminal.Terminal
	DeterministicOutput bool // true to disable timing-based rendering
	RenderOnDirty       bool // true to always render frames when marked dirty
}

func (opts Options) WithIsInteractive(isInteractive bool) Options {
	opts.IsInteractive = isInteractive
	return opts
}
