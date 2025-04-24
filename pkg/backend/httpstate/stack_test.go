// Copyright 2016-2025, Pulumi Corporation.
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

package httpstate

import (
	"fmt"
	"io"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestCloudBackendReference(t *testing.T) {
	t.Parallel()
	t.Run("string()", func(t *testing.T) {
		project := &workspace.Project{
			Name: "cbr-project",
		}
		defaultOrg := "cbr-default-org"
		stackName := tokens.MustParseStackName("cbr-test-stack")

		t.Run("elides default org if owner match", func(t *testing.T) {
			t.Parallel()
			// GIVEN

			// By populating the defaultOrg, we should not make any calls to the underlying client,
			// preferring what has already been configured.
			stubClient := &client.Client{}
			backend := &cloudBackend{
				client:         stubClient,
				currentProject: project,
				d:              diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			}

			ref := cloudBackendReference{
				name:       stackName,
				project:    tokens.Name(project.Name.String()),
				defaultOrg: defaultOrg,
				owner:      defaultOrg,
				b:          backend,
			}

			// WHEN
			stack := ref.String()

			// THEN
			assert.Equal(t, stackName.String(), stack)
		})

		t.Run("does not elide default org if does not match owner", func(t *testing.T) {
			t.Parallel()
			// GIVEN

			// By populating the defaultOrg, we should not make any calls to the underlying client,
			// preferring what has already been configured.
			stubClient := &client.Client{}
			backend := &cloudBackend{
				client:         stubClient,
				currentProject: project,
				d:              diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			}

			someOtherOrg := "some-other-org"

			ref := cloudBackendReference{
				name:       stackName,
				project:    tokens.Name(project.Name.String()),
				defaultOrg: defaultOrg,
				owner:      someOtherOrg,
				b:          backend,
			}

			// WHEN
			stack := ref.String()

			// THEN
			assert.Equal(t, fmt.Sprintf("%s/%s", someOtherOrg, stackName), stack)
		})

		t.Run("does not elide if projects do not match", func(t *testing.T) {
			t.Parallel()
			// GIVEN

			// By populating the defaultOrg, we should not make any calls to the underlying client,
			// preferring what has already been configured.
			stubClient := &client.Client{}
			backend := &cloudBackend{
				client:         stubClient,
				currentProject: project,
				d:              diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
			}

			otherProject := &workspace.Project{
				Name: "cbr-project-other",
			}

			ref := cloudBackendReference{
				name:       stackName,
				project:    tokens.Name(otherProject.Name.String()),
				defaultOrg: defaultOrg,
				owner:      defaultOrg,
				b:          backend,
			}

			// WHEN
			stack := ref.String()

			// THEN
			assert.Equal(t, fmt.Sprintf("%s/%s/%s", defaultOrg, otherProject.Name, stackName), stack)
		})
	})
}
