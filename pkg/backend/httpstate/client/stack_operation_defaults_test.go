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

package client

import (
	"net/http"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestStackOperationDefaultsResponses(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, body string
		want       apitype.StackOperationDefaults
	}{
		{
			name: "defaults",
			body: `{"updateID":"update",
 "stackOperationDefaults":{"refresh":true,"continueOnError":false,"runProgram":true,"parallel":2}}`,
			want: apitype.StackOperationDefaults{
				Refresh: new(true), ContinueOnError: new(false), RunProgram: new(true), Parallel: new(2),
			},
		},
		{name: "older server", body: `{"updateID":"update"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := newMockServer(http.StatusOK, tt.body)
			defer server.Close()
			client := NewClient(server.URL, "", true, nil)
			stack := StackIdentifier{Owner: "owner", Project: "project", Stack: tokens.MustParseStackName("stack")}
			proj := &workspace.Project{Name: "project", Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil)}
			_, details, err := client.CreateUpdate(t.Context(), apitype.UpdateUpdate, stack, proj, nil,
				apitype.UpdateMetadata{}, engine.UpdateOptions{}, false)
			require.NoError(t, err)
			require.Equal(t, tt.want, details.StackOperationDefaults)
			response, err := client.BeginUpdate(t.Context(), apitype.UpdateUpdate, stack, proj, nil,
				apitype.UpdateMetadata{}, engine.UpdateOptions{}, nil, false)
			require.NoError(t, err)
			require.Equal(t, tt.want, response.StackOperationDefaults)
		})
	}
}
