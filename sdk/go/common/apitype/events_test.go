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

package apitype

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperationResultFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want OperationResult
	}{
		{name: "nil", err: nil, want: OperationResultSucceeded},
		{name: "canceled", err: context.Canceled, want: OperationResultCanceled},
		{
			name: "wrapped canceled",
			err:  fmt.Errorf("op aborted: %w", context.Canceled),
			want: OperationResultCanceled,
		},
		{name: "failed", err: errors.New("boom"), want: OperationResultFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OperationResultFromError(tt.err))
		})
	}
}
