// Copyright 2026, Pulumi Corporation.
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

package ui

import (
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
)

// OperationResult is the high-level outcome of a stack operation.
type OperationResult string

const (
	OperationResultSucceeded OperationResult = "succeeded"
	OperationResultFailed    OperationResult = "failed"
	OperationResultCanceled  OperationResult = "canceled"
)

// OperationSummaryJSON is a structured summary of a stack operation suitable for JSON output.
type OperationSummaryJSON struct {
	// Result is the high-level outcome of the operation (succeeded, failed, canceled).
	Result OperationResult `json:"result"`

	// Duration records how long the operation took.
	Duration time.Duration `json:"duration"`

	// ChangeSummary contains a map of count per operation (create, update, etc).
	ChangeSummary display.ResourceChanges `json:"changeSummary,omitempty"`
}

// OperationSummarySink collects summary information about a stack operation.
// Callers can populate this while the operation runs and then use PrintOperationSummaryJSON
// at the CLI layer once the command has completed.
type OperationSummarySink struct {
	StartTime    time.Time
	EndTime      time.Time
	ChangeSummary display.ResourceChanges
	Err          error
	Canceled     bool
}

// OperationSummary builds an OperationSummaryJSON from the sink.
func (s *OperationSummarySink) OperationSummary() OperationSummaryJSON {
	result := OperationResultSucceeded
	if s.Canceled {
		result = OperationResultCanceled
	} else if s.Err != nil {
		result = OperationResultFailed
	}

	return OperationSummaryJSON{
		Result:       result,
		Duration:     s.EndTime.Sub(s.StartTime),
		ChangeSummary: s.ChangeSummary,
	}
}

// PrintOperationSummaryJSON prints a structured JSON summary for a completed operation.
func PrintOperationSummaryJSON(s *OperationSummarySink) error {
	if s == nil {
		return nil
	}
	summary := s.OperationSummary()
	return PrintJSON(summary)
}

