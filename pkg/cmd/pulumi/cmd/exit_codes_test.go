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

package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

func TestExitCodeFor_KnownErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: ExitSuccess},
		{
			name: "bail error",
			err:  result.BailError(errors.New("bail")),
			want: ExitCodeError,
		},
		{
			name: "login required",
			err:  backenderr.LoginRequiredError{},
			want: ExitAuthenticationError,
		},
		{
			name: "forbidden",
			err:  backenderr.ForbiddenError{},
			want: ExitAuthenticationError,
		},
		{
			name: "missing env var for non-interactive login",
			err:  backenderr.MissingEnvVarForNonInteractiveError{},
			want: ExitAuthenticationError,
		},
		{
			name: "stack not found",
			err:  backenderr.StackNotFoundError{StackName: "missing"},
			want: ExitStackNotFound,
		},
		{
			name: "no stacks",
			err:  backenderr.NoStacksError{},
			want: ExitStackNotFound,
		},
		{
			name: "no stack selected",
			err:  backenderr.NoStackSelectedError{},
			want: ExitStackNotFound,
		},
		{
			name: "stack state not found",
			err:  backenderr.StackStateNotFoundError{StackName: "missing"},
			want: ExitStackNotFound,
		},
		{
			name: "cancelled update",
			err:  backenderr.CancelledError{Operation: "update"},
			want: ExitCancelled,
		},
		{
			name: "no changes expected",
			err:  backenderr.NoChangesExpectedError{Operation: "preview"},
			want: ExitNoChanges,
		},
		{
			name: "no confirmation in non-interactive",
			err:  backenderr.NoConfirmationInNonInteractiveError{},
			want: ExitConfigurationError,
		},
		{
			name: "wrapped auth error",
			err:  wrap(errors.New("outer"), backenderr.LoginRequiredError{}),
			want: ExitAuthenticationError,
		},
		{
			name: "api caller error preserves exit code",
			err:  &cloud.APIError{ExitCode: cmdutil.ExitCodeError},
			want: ExitCodeError,
		},
		{
			name: "api auth error preserves exit code",
			err:  &cloud.APIError{ExitCode: cmdutil.ExitAuthenticationError},
			want: ExitAuthenticationError,
		},
		{
			name: "api cancelled preserves exit code",
			err:  &cloud.APIError{ExitCode: cmdutil.ExitCancelled},
			want: ExitCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCodeFor(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func wrap(outer error, inner error) error {
	return errors.Join(outer, inner)
}
