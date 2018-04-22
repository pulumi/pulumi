// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

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
	FirstDisplayTime() int
	SetFirstDisplayTime(time int)

	ColorizedColumns() []string
	ColorizedSuffix() string
	HideRowIfUnnecessary() bool
}

type ResourceRow interface {
	Row

	Step() engine.StepEventMetadata
	SetStep(step engine.StepEventMetadata)

	// The tick we were on when we created this row.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	Tick() int

	Done() bool
	SetDone()

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

func (data *headerRowData) FirstDisplayTime() int {
	// sort the header before all other rows
	return -1
}

func (data *headerRowData) SetFirstDisplayTime(time int) {
	// Nothing to do here.   Header is always at the same time.
}

func (data *headerRowData) ColorizedColumns() []string {
	if len(data.columns) == 0 {
		blue := func(msg string) string {
			return colors.Blue + msg + colors.Reset
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
		data.columns = []string{header("Resource Type"), header("Name"), statusColumn, header("Extra Info")}
	}

	return data.columns
}

func (data *headerRowData) ColorizedSuffix() string {
	return ""
}

// Implementation of a row used for all the resource rows in the grid.
type resourceRowData struct {
	firstDisplayTime int

	display *ProgressDisplay

	// The change that the engine wants apply to that resource.
	step engine.StepEventMetadata

	// The tick we were on when we created this row.  Purely used for generating an
	// ellipses to show progress for in-flight resources.
	tick int

	// If the engine finished processing this resources.
	done bool

	// If we failed this operation for any reason.
	failed bool

	diagInfo *DiagInfo

	// If this row should be hidden by default.  We will hide unless we have any child nodes
	// we need to show.
	hideRowIfUnnecessary bool
}

func (data *resourceRowData) FirstDisplayTime() int {
	// sort the header before all other rows
	return data.firstDisplayTime
}

func (data *resourceRowData) SetFirstDisplayTime(time int) {
	// only set this if it's the first time.
	if data.firstDisplayTime == 0 {
		data.firstDisplayTime = time
	}
}

func (data *resourceRowData) HideRowIfUnnecessary() bool {
	return data.hideRowIfUnnecessary
}

func (data *resourceRowData) Step() engine.StepEventMetadata {
	return data.step
}

func (data *resourceRowData) SetStep(step engine.StepEventMetadata) {
	data.step = step
}

func (data *resourceRowData) Tick() int {
	return data.tick
}

func (data *resourceRowData) Done() bool {
	return data.done
}

func (data *resourceRowData) SetDone() {
	data.done = true
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

	switch payload.Severity {
	case diag.Error:
		diagInfo.ErrorCount++
		diagInfo.LastError = &event
	case diag.Warning:
		diagInfo.WarningCount++
		diagInfo.LastWarning = &event
	case diag.Infoerr:
		diagInfo.InfoCount++
		diagInfo.LastInfoError = &event
	case diag.Info:
		diagInfo.InfoCount++
		diagInfo.LastInfo = &event
	case diag.Debug:
		diagInfo.DebugCount++
		diagInfo.LastDebug = &event
	}

	diagInfo.DiagEvents = append(diagInfo.DiagEvents, event)
}

type column int

const (
	typeColumn   column = 0
	nameColumn   column = 1
	statusColumn column = 2
	infoColumn   column = 3
)

func (data *resourceRowData) ColorizedSuffix() string {
	if !data.display.Done && !data.done {
		suffixes := data.display.suffixesArray
		ellipses := suffixes[(data.tick+data.display.currentTick)%len(suffixes)]

		return data.step.Op.Color() + ellipses + colors.Reset
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

	columns := make([]string, 4)
	columns[typeColumn] = typ
	columns[nameColumn] = name

	diagInfo := data.diagInfo

	if data.done {
		failed := data.failed || diagInfo.ErrorCount > 0
		columns[statusColumn] = data.display.getStepDoneDescription(step, failed)
	} else {
		columns[statusColumn] = data.display.getStepInProgressDescription(step)
	}

	columns[infoColumn] = data.getInfo()
	return columns
}

func (data *resourceRowData) getInfo() string {
	step := data.step
	changesBuf := &bytes.Buffer{}

	if step.Old != nil && step.New != nil && step.Old.Inputs != nil && step.New.Inputs != nil {
		diff := step.Old.Inputs.Diff(step.New.Inputs)

		if diff != nil {
			writeString(changesBuf, "changes:")

			updates := make(resource.PropertyMap)
			for k := range diff.Updates {
				updates[k] = resource.PropertyValue{}
			}

			writePropertyKeys(changesBuf, diff.Adds, deploy.OpCreate)
			writePropertyKeys(changesBuf, diff.Deletes, deploy.OpDelete)
			writePropertyKeys(changesBuf, updates, deploy.OpReplace)
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
	} else if diagInfo.ErrorCount > 1 {
		appendDiagMessage(fmt.Sprintf("%v debug messages", diagInfo.DebugCount))
	}

	// If we're not totally done, also print out the worst diagnostic next to the status message.
	// This is helpful for long running tasks to know what's going on.  However, once done, we print
	// the diagnostics at the bottom, so we don't need to show this.
	worstDiag := getWorstDiagnostic(data.diagInfo)
	if worstDiag != nil && !data.display.Done {
		eventMsg := data.display.renderProgressDiagEvent(*worstDiag)
		if eventMsg != "" {
			diagMsg += ". " + eventMsg
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
func getWorstDiagnostic(diagInfo *DiagInfo) *engine.Event {
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
