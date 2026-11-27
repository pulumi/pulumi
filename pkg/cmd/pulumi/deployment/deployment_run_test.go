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

package deployment

import (
	"errors"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
)

func TestDeploymentRunErrorWithoutSettings(t *testing.T) {
	t.Parallel()

	apiErr := &apitype.ErrorResponse{
		Code: 400,
		Message: "Bad Request: no deployment settings found for program " +
			"6a9b5bb6-85ab-4c8a-8c16-9d6f1c0c1249, either configure deployment settings " +
			"for the stack or set inheritSettings: false in your request body",
	}
	err := deploymentRunError(
		fmt.Errorf("creating deployment failed: %w", apiErr),
		"myorg/myproj/dev",
		apitype.Update,
		"",
		RemoteArgs{InheritSettings: true},
	)

	require.EqualError(t, err, "no deployment settings are configured for stack myorg/myproj/dev.\n"+
		"Configure them with 'pulumi deployment settings edit', or pass a repo URL: "+
		"'pulumi deployment run update <git-url>'.")
}

func TestDeploymentRunErrorPreservesOtherErrors(t *testing.T) {
	t.Parallel()

	noSettingsErr := fmt.Errorf("creating deployment failed: %w", &apitype.ErrorResponse{
		Code:    400,
		Message: "Bad Request: no deployment settings found for program program-id",
	})
	otherBadRequest := fmt.Errorf("creating deployment failed: %w", &apitype.ErrorResponse{
		Code:    400,
		Message: "Bad Request: another validation failed",
	})
	serverErr := fmt.Errorf("creating deployment failed: %w", &apitype.ErrorResponse{
		Code:    500,
		Message: "Internal Server Error",
	})
	otherErr := errors.New("another error")

	tests := map[string]struct {
		err  error
		url  string
		args RemoteArgs
	}{
		"explicit repo URL": {
			err:  noSettingsErr,
			url:  "https://github.com/myorg/myrepo",
			args: RemoteArgs{InheritSettings: true},
		},
		"settings not inherited": {
			err:  noSettingsErr,
			args: RemoteArgs{InheritSettings: false},
		},
		"other bad request": {
			err:  otherBadRequest,
			args: RemoteArgs{InheritSettings: true},
		},
		"server error": {
			err:  serverErr,
			args: RemoteArgs{InheritSettings: true},
		},
		"non-API error": {
			err:  otherErr,
			args: RemoteArgs{InheritSettings: true},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := deploymentRunError(tt.err, "myorg/myproj/dev", apitype.Update, tt.url, tt.args)
			require.ErrorIs(t, err, tt.err)
		})
	}
}
