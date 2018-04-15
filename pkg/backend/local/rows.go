// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"

	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

type Row interface {
	ColorizedColumns() []string
	UncolorizedColumns() []string

	ColorizedSuffix() string
}

type ResourceRow interface {
	Row

	// The change that the engine wants apply to that resource.
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
	columns            []string
	uncolorizedColumns []string
}

func (data *headerRowData) ColorizedColumns() []string {
	if len(data.columns) == 0 {
		blue := func(msg string) string {
			return colors.Blue + msg + colors.Reset
		}

		header := func(msg string) string {
			return blue(msg)
		}

		data.columns = []string{"#", header("Resource Type"), header("Name"), header("Status"), header("Extra Info")}
	}

	return data.columns
}

func (data *headerRowData) UncolorizedColumns() []string {
	if len(data.uncolorizedColumns) == 0 {
		data.uncolorizedColumns = uncolorise(data.ColorizedColumns())
	}

	return data.uncolorizedColumns
}

func (data *headerRowData) ColorizedSuffix() string {
	return ""
}

// Implementation of a row used for all the resource rows in the grid.
type resourceRowData struct {
	display *ProgressDisplay

	// The simple short ID we have generated for the resource to present it to the user.
	// Usually similar to the form: aws.Function("name")
	id string

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

	columns            []string
	uncolorizedColumns []string
}

func (data *resourceRowData) SetStep(step engine.StepEventMetadata) {
	data.step = step
	data.ClearCachedData()
}

func (data *resourceRowData) Tick() int {
	return data.tick
}

func (data *resourceRowData) Done() bool {
	return data.done
}

func (data *resourceRowData) SetDone() {
	data.done = true
	data.ClearCachedData()
}

func (data *resourceRowData) Failed() bool {
	return data.failed
}

func (data *resourceRowData) SetFailed() {
	data.failed = true
	data.ClearCachedData()
}

func (data *resourceRowData) DiagInfo() *DiagInfo {
	return data.diagInfo
}

func (data *resourceRowData) RecordDiagEvent(diagEvent engine.Event) {
	combineDiagnosticInfo(data.diagInfo, diagEvent)
	data.ClearCachedData()
}

func (data *resourceRowData) ClearCachedData() {
	data.columns = []string{}
	data.uncolorizedColumns = []string{}
}

type column int

const (
	idColumn     column = 0
	typeColumn   column = 1
	nameColumn   column = 2
	statusColumn column = 3
	infoColumn   column = 4
)

var ellipsesArray = []string{"", ".", "..", "..."}

func (data *resourceRowData) ColorizedSuffix() string {
	if !data.done {
		uncolorizedColumns := data.UncolorizedColumns()
		ellipses := ellipsesArray[(data.tick+data.display.currentTick)%len(ellipsesArray)]

		if uncolorizedColumns[infoColumn] == "" {
			return data.step.Op.Color() + ellipses + colors.Reset
		}

		return ellipses
	}

	return ""
}

func (data *resourceRowData) ColorizedColumns() []string {
	if len(data.columns) == 0 {
		columns := data.getUnpaddedColumns()
		data.columns = columns
	}

	return data.columns
}

// Gets the single line summary to show for a resource.  This will include the current state of
// the resource (i.e. "Creating", "Replaced", "Failed", etc.) as well as relevant diagnostic
// information if there is any.
func (data *resourceRowData) getUnpaddedColumns() []string {
	step := data.step
	if step.Op == "" {
		contract.Failf("Finishing a resource we never heard about: '%s'", data.id)
	}

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
	columns[idColumn] = data.id
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

func uncolorise(columns []string) []string {
	uncolorizedColumns := make([]string, len(columns))

	for i, v := range columns {
		uncolorizedColumns[i] = colors.Never.Colorize(v)
	}

	return uncolorizedColumns
}

func (data *resourceRowData) UncolorizedColumns() []string {
	if len(data.uncolorizedColumns) == 0 {
		columns := data.ColorizedColumns()
		data.uncolorizedColumns = uncolorise(columns)
	}

	return data.uncolorizedColumns
}
