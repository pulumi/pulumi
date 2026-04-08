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
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/stretchr/testify/assert"
)

func TestOperationSummary(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Second)

	tests := []struct {
		name           string
		sink           OperationSummarySink
		expectedResult OperationResult
		expectedDur    time.Duration
	}{
		{
			name: "succeeded",
			sink: OperationSummarySink{
				StartTime:     start,
				EndTime:       end,
				ChangeSummary: display.ResourceChanges{"create": 1},
			},
			expectedResult: OperationResultSucceeded,
			expectedDur:    5 * time.Second,
		},
		{
			name: "failed",
			sink: OperationSummarySink{
				StartTime: start,
				EndTime:   end,
				Err:       errors.New("something went wrong"),
			},
			expectedResult: OperationResultFailed,
			expectedDur:    5 * time.Second,
		},
		{
			name: "canceled",
			sink: OperationSummarySink{
				StartTime: start,
				EndTime:   end,
				Err:       context.Canceled,
			},
			expectedResult: OperationResultCanceled,
			expectedDur:    5 * time.Second,
		},
		{
			name: "wrapped canceled",
			sink: OperationSummarySink{
				StartTime: start,
				EndTime:   end,
				Err:       fmt.Errorf("op failed: %w", context.Canceled),
			},
			expectedResult: OperationResultCanceled,
			expectedDur:    5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			summary := tt.sink.OperationSummary()
			assert.Equal(t, tt.expectedResult, summary.Result)
			assert.Equal(t, tt.expectedDur, summary.Duration)
		})
	}
}
