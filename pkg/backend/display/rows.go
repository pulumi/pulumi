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
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dustin/go-humanize/english"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	PolicyPayloads() []engine.PolicyViolationEventPayload

	RecordDiagEvent(diagEvent engine.Event)
	RecordPolicyViolationEvent(diagEvent engine.Event)
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
		header := func(msg string) string {
			return columnHeader(msg)
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

// resourceRowData is the implementation of a row used for all the resource rows in the grid.
type resourceRowData struct {
	displayOrderIndex int

	display *ProgressDisplay

	// The change that the engine wants apply to that resource.
	step        engine.StepEventMetadata
	outputSteps []engine.StepEventMetadata

	// The tick we were on when we created this row.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	tick int

	// If we failed this operation for any reason.
	failed bool

	diagInfo       *DiagInfo
	policyPayloads []engine.PolicyViolationEventPayload

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
	payload := event.Payload().(engine.DiagEventPayload)
	data.recordDiagEventPayload(payload)
}

func (data *resourceRowData) recordDiagEventPayload(payload engine.DiagEventPayload) {
	diagInfo := data.diagInfo
	diagInfo.LastDiag = &payload

	if payload.Severity == diag.Error {
		diagInfo.LastError = &payload
	}

	if diagInfo.StreamIDToDiagPayloads == nil {
		diagInfo.StreamIDToDiagPayloads = make(map[int32][]engine.DiagEventPayload)
	}

	payloads := diagInfo.StreamIDToDiagPayloads[payload.StreamID]
	payloads = append(payloads, payload)
	diagInfo.StreamIDToDiagPayloads[payload.StreamID] = payloads

	if !payload.Ephemeral {
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

// PolicyInfo returns the PolicyInfo object associated with the resourceRowData.
func (data *resourceRowData) PolicyPayloads() []engine.PolicyViolationEventPayload {
	return data.policyPayloads
}

// RecordPolicyViolationEvent records a policy event with the resourceRowData.
func (data *resourceRowData) RecordPolicyViolationEvent(event engine.Event) {
	pePayload := event.Payload().(engine.PolicyViolationEventPayload)
	data.policyPayloads = append(data.policyPayloads, pePayload)
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

	if isRootStack(data.step) {
		// the root stack only becomes 'done' once the program has completed (i.e. the condition
		// checked just above this).  If the program is not finished, then always show the root
		// stack as not done so the user sees "running..." presented for it.
		return false
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

	urn := data.step.URN
	if urn == "" {
		// If we don't have a URN yet, mock parent it to the global stack.
		urn = resource.DefaultRootStackURN(data.display.stack, data.display.proj)
	}
	name := string(urn.Name())
	typ := simplifyTypeName(urn.Type())

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
	switch step.Op {
	case deploy.OpCreateReplacement, deploy.OpDeleteReplaced:
		// if we're doing a replacement, see if we can find a replace step that contains useful
		// information to display.
		for _, outputStep := range data.outputSteps {
			if outputStep.Op == deploy.OpReplace {
				step = outputStep
			}
		}

	case deploy.OpImport, deploy.OpImportReplacement:
		// If we're doing an import, see if we have the imported state to diff.
		for _, outputStep := range data.outputSteps {
			if outputStep.Op == step.Op {
				step = outputStep
			}
		}
	}

	var diagMsg string
	appendDiagMessage := func(msg string) {
		if diagMsg != "" {
			diagMsg += "; "
		}

		diagMsg += msg
	}

	changes := getDiffInfo(step, data.display.action)
	if colors.Never.Colorize(changes) != "" {
		appendDiagMessage("[" + changes + "]")
	}

	diagInfo := data.diagInfo
	if data.display.done {
		// If we are done, show a summary of how many messages were printed.
		if c := diagInfo.ErrorCount; c > 0 {
			appendDiagMessage(fmt.Sprintf("%d %s%s%s",
				c, colors.SpecError, english.PluralWord(c, "error", ""), colors.Reset))
		}
		if c := diagInfo.WarningCount; c > 0 {
			appendDiagMessage(fmt.Sprintf("%d %s%s%s",
				c, colors.SpecWarning, english.PluralWord(c, "warning", ""), colors.Reset))
		}
		if c := diagInfo.InfoCount; c > 0 {
			appendDiagMessage(fmt.Sprintf("%d %s%s%s",
				c, colors.SpecInfo, english.PluralWord(c, "message", ""), colors.Reset))
		}
		if c := diagInfo.DebugCount; c > 0 {
			appendDiagMessage(fmt.Sprintf("%d %s%s%s",
				c, colors.SpecDebug, english.PluralWord(c, "debug", ""), colors.Reset))
		}
	} else {
		// If we're not totally done, and we're in the tree-view, just print out the last error (if
		// there is one) next to the status message. This is helpful for long running tasks to know
		// something bad has happened. However, once done, we print the diagnostics at the bottom, so we don't
		// need to show this.
		//
		// if we're not in the tree-view (i.e. non-interactive mode), then we want to print out
		// whatever the last diagnostics was that we got.  This way, as we're hearing about
		// diagnostic events, we're always printing out the last one.

		diagnostic := data.diagInfo.LastDiag
		if data.display.isTerminal && data.diagInfo.LastError != nil {
			diagnostic = data.diagInfo.LastError
		}

		if diagnostic != nil {
			eventMsg := data.display.renderProgressDiagEvent(*diagnostic, true /*includePrefix:*/)
			if eventMsg != "" {
				appendDiagMessage(eventMsg)
			}
		}
	}

	newLineIndex := strings.Index(diagMsg, "\n")
	if newLineIndex >= 0 {
		diagMsg = diagMsg[0:newLineIndex]
	}

	return diagMsg
}

func getDiffInfo(step engine.StepEventMetadata, action apitype.UpdateKind) string {
	diffOutputs := action == apitype.RefreshUpdate
	changesBuf := &bytes.Buffer{}
	if step.Old != nil && step.New != nil {
		var diff *resource.ObjectDiff
		if step.DetailedDiff != nil {
			diff = translateDetailedDiff(step)
		} else if diffOutputs {
			if step.Old.Outputs != nil && step.New.Outputs != nil {
				diff = step.Old.Outputs.Diff(step.New.Outputs)
			}
		} else if step.Old.Inputs != nil && step.New.Inputs != nil {
			diff = step.Old.Inputs.Diff(step.New.Inputs)
		}

		// Show a diff if either `provider` or `protect` changed; they might not show a diff via inputs or outputs, but
		// it is still useful to show that these changed in output.
		recordMetadataDiff := func(name string, old, new resource.PropertyValue) {
			if old != new {
				if diff == nil {
					diff = &resource.ObjectDiff{
						Adds:    make(resource.PropertyMap),
						Deletes: make(resource.PropertyMap),
						Sames:   make(resource.PropertyMap),
						Updates: make(map[resource.PropertyKey]resource.ValueDiff),
					}
				}

				diff.Updates[resource.PropertyKey(name)] = resource.ValueDiff{Old: old, New: new}
			}
		}

		recordMetadataDiff("provider",
			resource.NewStringProperty(step.Old.Provider), resource.NewStringProperty(step.New.Provider))
		recordMetadataDiff("protect",
			resource.NewBoolProperty(step.Old.Protect), resource.NewBoolProperty(step.New.Protect))

		if diff != nil {
			writeString(changesBuf, "diff: ")

			updates := make(resource.PropertyMap)
			for k := range diff.Updates {
				updates[k] = resource.PropertyValue{}
			}

			filteredKeys := func(m resource.PropertyMap) []string {
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, string(k))
				}
				return keys
			}
			if include := step.Diffs; include != nil {
				includeSet := make(map[resource.PropertyKey]bool)
				for _, k := range include {
					includeSet[k] = true
				}
				filteredKeys = func(m resource.PropertyMap) []string {
					var filteredKeys []string
					for k := range m {
						if includeSet[k] {
							filteredKeys = append(filteredKeys, string(k))
						}
					}
					return filteredKeys
				}
			}

			writePropertyKeys(changesBuf, filteredKeys(diff.Adds), deploy.OpCreate)
			writePropertyKeys(changesBuf, filteredKeys(diff.Deletes), deploy.OpDelete)
			writePropertyKeys(changesBuf, filteredKeys(updates), deploy.OpUpdate)
		}
	}

	fprintIgnoreError(changesBuf, colors.Reset)
	return changesBuf.String()
}

func writePropertyKeys(b io.StringWriter, keys []string, op deploy.StepOp) {
	if len(keys) > 0 {
		writeString(b, strings.Trim(op.Prefix(), " "))

		sort.Strings(keys)

		for index, k := range keys {
			if index != 0 {
				writeString(b, ",")
			}
			writeString(b, k)
		}

		writeString(b, colors.Reset)
	}
}
