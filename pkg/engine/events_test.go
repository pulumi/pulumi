// Copyright 2020, Pulumi Corporation.
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

package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/assert"
)

func TestTrySendEvent(t *testing.T) {
	t.Parallel()
	e := Event{}
	c := make(chan Event, 100)
	assert.Equal(t, true, trySendEvent(c, e))
	close(c)
	assert.Equal(t, false, trySendEvent(c, e))
}

func TestTryCloseEventChan(t *testing.T) {
	t.Parallel()
	c := make(chan Event, 100)
	assert.Equal(t, true, tryCloseEventChan(c))
	assert.Equal(t, false, tryCloseEventChan(c))
}

func TestOperationResultFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want apitype.OperationResult
	}{
		{name: "nil", err: nil, want: apitype.OperationResultSucceeded},
		{name: "canceled", err: context.Canceled, want: apitype.OperationResultCanceled},
		{
			name: "wrapped canceled",
			err:  fmt.Errorf("op aborted: %w", context.Canceled),
			want: apitype.OperationResultCanceled,
		},
		{name: "failed", err: errors.New("boom"), want: apitype.OperationResultFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, operationResultFromError(tt.err))
		})
	}
}
