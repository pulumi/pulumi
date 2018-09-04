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

package filestate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
)

type Row interface {
	DisplayOrderIndex() int
	SetDisplayOrderIndex(index int)

	ColorizedColumns() []string
	ColorizedSuffix() string

	HideRowIfUnnecessary() bool
	SetHideRowIfUnnecessary(value bool)
}

type ResourceRow interface {
	Row

	Step() engine.StepEventMetadata
	SetStep(step engine.StepEventMetadata)
	AddOutputStep(step engine.StepEventMetadata)

	// The tick we were on when we created this row.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	Tick() int

	IsDone() bool

	SetFailed()

	DiagInfo() *DiagInfo
	RecordDiagEvent(diagEvent engine.Event)
}

// Implementation of a Row, used for the header of the grid.
type headerRowData struct {
	display *ProgressDisplay
	columns []string
}

func (data *headerRowData) HideRowIfUnnecessary() bool {
	return false
}

func (data *headerRowData) SetHideRowIfUnnecessary(value bool) {
}

func (data *headerRowData) DisplayOrderIndex() int {
	// sort the header before all other rows
	return -1
}

func (data *headerRowData) SetDisplayOrderIndex(time int) {
	// Nothing to do here.   Header is always at the same index.
}

func (data *headerRowData) ColorizedColumns() []string {
	if len(data.columns) == 0 {
		blue := func(msg string) string {
			return colors.BrightBlue + msg + colors.Reset
		}

		header := func(msg string) string {
			return blue(msg)
		}

		var statusColumn string
		if data.display.isPreview {
			statusColumn = header("Plan")
		} else {
			statusColumn = header("Status")
		}
		data.columns = []string{"", header("Type"), header("Name"), statusColumn, header("Info")}
	}

	return data.columns
}

func (data *headerRowData) ColorizedSuffix() string {
	return ""
}

// Implementation of a row used for all the resource rows in the grid.
type resourceRowData struct {
	displayOrderIndex int

	display *ProgressDisplay

	// The change that the engine wants apply to that resource.
	step        engine.StepEventMetadata
	outputSteps []engine.StepEventMetadata

	// True if we should diff outputs instead of inputs for this row.
	diffOutputs bool

	// The tick we were on when we created this row.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	tick int

	// If we failed this operation for any reason.
	failed bool

	diagInfo *DiagInfo

	// If this row should be hidden by default.  We will hide unless we have any child nodes
	// we need to show.
	hideRowIfUnnecessary bool
}

func (data *resourceRowData) DisplayOrderIndex() int {
	// sort the header before all other rows
	return data.displayOrderIndex
}

func (data *resourceRowData) SetDisplayOrderIndex(index int) {
	// only set this if it's the first time.
	if data.displayOrderIndex == 0 {
		data.displayOrderIndex = index
	}
}

func (data *resourceRowData) HideRowIfUnnecessary() bool {
	return data.hideRowIfUnnecessary
}

func (data *resourceRowData) SetHideRowIfUnnecessary(value bool) {
	data.hideRowIfUnnecessary = value
}

func (data *resourceRowData) Step() engine.StepEventMetadata {
	return data.step
}

func (data *resourceRowData) SetStep(step engine.StepEventMetadata) {
	data.step = step
	if step.Op == deploy.OpRefresh {
		data.diffOutputs = true
	}
}

func (data *resourceRowData) AddOutputStep(step engine.StepEventMetadata) {
	data.outputSteps = append(data.outputSteps, step)
}

func (data *resourceRowData) Tick() int {
	return data.tick
}

func (data *resourceRowData) Failed() bool {
	return data.failed
}

func (data *resourceRowData) SetFailed() {
	data.failed = true
}

func (data *resourceRowData) DiagInfo() *DiagInfo {
	return data.diagInfo
}

func (data *resourceRowData) RecordDiagEvent(event engine.Event) {
	diagInfo := data.diagInfo
	payload := event.Payload.(engine.DiagEventPayload)

	diagInfo.LastDiag = &payload

	switch payload.Severity {
	case diag.Error:
		diagInfo.LastError = &payload
	case diag.Warning:
		diagInfo.LastWarning = &payload
	case diag.Infoerr:
		diagInfo.LastInfoError = &payload
	case diag.Info:
		diagInfo.LastInfo = &payload
	case diag.Debug:
		diagInfo.LastDebug = &payload
	}

	if diagInfo.StreamIDToDiagPayloads == nil {
		diagInfo.StreamIDToDiagPayloads = make(map[int32][]engine.DiagEventPayload)
	}

	payloads := diagInfo.StreamIDToDiagPayloads[payload.StreamID]

	// Record the count if this is for the default stream, or this is the first event in a a
	// non-default stream
	recordCount := payload.StreamID == 0 || len(payloads) == 0

	payloads = append(payloads, payload)
	diagInfo.StreamIDToDiagPayloads[payload.StreamID] = payloads

	if recordCount && !payload.Ephemeral {
		switch payload.Severity {
		case diag.Error:
			diagInfo.ErrorCount++
		case diag.Warning:
			diagInfo.WarningCount++
		case diag.Infoerr:
			diagInfo.InfoCount++
		case diag.Info:
			diagInfo.InfoCount++
		case diag.Debug:
			diagInfo.DebugCount++
		}
	}
}

type column int

const (
	opColumn     column = 0
	typeColumn   column = 1
	nameColumn   column = 2
	statusColumn column = 3
	infoColumn   column = 4
)

func (data *resourceRowData) IsDone() bool {
	if data.failed {
		// consider a failed resource 'done'.
		return true
	}

	if data.display.done {
		// if the display is done, then we're definitely done.
		return true
	}

	// We're done if we have the output-step for whatever step operation we're performing
	return data.ContainsOutputsStep(data.step.Op)
}

func (data *resourceRowData) ContainsOutputsStep(op deploy.StepOp) bool {
	for _, s := range data.outputSteps {
		if s.Op == op {
			return true
		}
	}

	return false
}

func (data *resourceRowData) ColorizedSuffix() string {
	if !data.IsDone() && data.display.isTerminal {
		op := data.display.getStepOp(data.step)
		if op != deploy.OpSame || isRootURN(data.step.URN) {
			suffixes := data.display.suffixesArray
			ellipses := suffixes[(data.tick+data.display.currentTick)%len(suffixes)]

			return op.Color() + ellipses + colors.Reset
		}
	}

	return ""
}

func (data *resourceRowData) ColorizedColumns() []string {
	step := data.step

	var name string
	var typ string
	if data.step.URN == "" {
		name = "global"
		typ = "global"
	} else {
		name = string(data.step.URN.Name())
		typ = simplifyTypeName(data.step.URN.Type())
	}

	columns := make([]string, 5)
	columns[opColumn] = data.display.getStepOpLabel(step)
	columns[typeColumn] = typ
	columns[nameColumn] = name

	diagInfo := data.diagInfo

	if data.IsDone() {
		failed := data.failed || diagInfo.ErrorCount > 0
		columns[statusColumn] = data.display.getStepDoneDescription(step, failed)
	} else {
		columns[statusColumn] = data.display.getStepInProgressDescription(step)
	}

	columns[infoColumn] = data.getInfoColumn()
	return columns
}

func (data *resourceRowData) getInfoColumn() string {
	step := data.step

	if step.Op == deploy.OpCreateReplacement || step.Op == deploy.OpDeleteReplaced {
		// if we're doing a replacement, see if we can find a replace step that contains useful
		// information to display.
		for _, outputStep := range data.outputSteps {
			if outputStep.Op == deploy.OpReplace {
				step = outputStep
			}
		}
	}

	changesBuf := &bytes.Buffer{}

	if step.Old != nil && step.New != nil {
		var diff *resource.ObjectDiff
		if data.diffOutputs {
			if step.Old.Outputs != nil && step.New.Outputs != nil {
				diff = step.Old.Outputs.Diff(step.New.Outputs)
			}
		} else if step.Old.Inputs != nil && step.New.Inputs != nil {
			diff = step.Old.Inputs.Diff(step.New.Inputs)
		}
		if step.Old.Provider != step.New.Provider {
			if diff == nil {
				diff = &resource.ObjectDiff{
					Adds:    make(resource.PropertyMap),
					Deletes: make(resource.PropertyMap),
					Sames:   make(resource.PropertyMap),
					Updates: make(map[resource.PropertyKey]resource.ValueDiff),
				}
			}
			diff.Updates["provider"] = resource.ValueDiff{
				Old: resource.NewStringProperty(step.Old.Provider),
				New: resource.NewStringProperty(step.New.Provider),
			}
		}

		if diff != nil {
			writeString(changesBuf, "changes:")

			updates := make(resource.PropertyMap)
			for k := range diff.Updates {
				updates[k] = resource.PropertyValue{}
			}

			writePropertyKeys(changesBuf, diff.Adds, deploy.OpCreate)
			writePropertyKeys(changesBuf, diff.Deletes, deploy.OpDelete)
			writePropertyKeys(changesBuf, updates, deploy.OpUpdate)
		}
	}

	fprintIgnoreError(changesBuf, colors.Reset)
	changes := changesBuf.String()

	diagMsg := ""

	if colors.Never.Colorize(changes) != "" {
		diagMsg += changes
	}

	appendDiagMessage := func(msg string) {
		if diagMsg != "" {
			diagMsg += ", "
		}

		diagMsg += msg
	}

	diagInfo := data.diagInfo

	if diagInfo.ErrorCount == 1 {
		appendDiagMessage("1 error")
	} else if diagInfo.ErrorCount > 1 {
		appendDiagMessage(fmt.Sprintf("%v errors", diagInfo.ErrorCount))
	}

	if diagInfo.WarningCount == 1 {
		appendDiagMessage("1 warning")
	} else if diagInfo.WarningCount > 1 {
		appendDiagMessage(fmt.Sprintf("%v warnings", diagInfo.WarningCount))
	}

	if diagInfo.InfoCount == 1 {
		appendDiagMessage("1 info message")
	} else if diagInfo.InfoCount > 1 {
		appendDiagMessage(fmt.Sprintf("%v info messages", diagInfo.InfoCount))
	}

	if diagInfo.DebugCount == 1 {
		appendDiagMessage("1 debug message")
	} else if diagInfo.DebugCount > 1 {
		appendDiagMessage(fmt.Sprintf("%v debug messages", diagInfo.DebugCount))
	}

	if !data.display.done {
		// If we're not totally done, and we're in the tree-view also print out the worst diagnostic
		// next to the status message. This is helpful for long running tasks to know what's going
		// on. However, once done, we print the diagnostics at the bottom, so we don't need to show
		// this.
		//
		// if we're not in the tree-view (i.e. non-interactive mode), then we want to print out
		// whatever the last diagnostics was that we got.  This way, as we're hearing about
		// diagnostic events, we're always printing out the last one.

		var diagnostic *engine.DiagEventPayload
		if data.display.isTerminal {
			diagnostic = data.diagInfo.LastDiag
		} else {
			diagnostic = getWorstDiagnostic(data.diagInfo)
		}

		if diagnostic != nil {
			eventMsg := data.display.renderProgressDiagEvent(*diagnostic, true /*includePrefix:*/)
			diagCount := diagInfo.DebugCount + diagInfo.ErrorCount + diagInfo.InfoCount + diagInfo.WarningCount
			if diagCount > 0 {
				diagMsg += ". "
			}

			if eventMsg != "" {
				diagMsg += eventMsg
			}
		}
	}

	newLineIndex := strings.Index(diagMsg, "\n")
	if newLineIndex >= 0 {
		diagMsg = diagMsg[0:newLineIndex]
	}

	return diagMsg
}

// Returns the worst diagnostic we've seen.  Used to produce a diagnostic string to go along with
// any resource if it has had any issues.
func getWorstDiagnostic(diagInfo *DiagInfo) *engine.DiagEventPayload {
	if diagInfo.LastError != nil {
		return diagInfo.LastError
	}

	if diagInfo.LastWarning != nil {
		return diagInfo.LastWarning
	}

	if diagInfo.LastInfoError != nil {
		return diagInfo.LastInfoError
	}

	if diagInfo.LastInfo != nil {
		return diagInfo.LastInfo
	}

	return diagInfo.LastDebug
}
