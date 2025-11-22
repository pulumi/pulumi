// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package show

import (
	"bytes"
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWs "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/require"
)

func TestShowCmd(t *testing.T) {
	ms := backend.MockStack{
		SnapshotF: func(ctx context.Context, secretsProvider secrets.Provider) (*deploy.Snapshot, error) {
			return &deploy.Snapshot{
				Manifest:       deploy.Manifest{},
				SecretsManager: &secrets.MockSecretsManager{},
				Resources: []*resource.State{
					{
						URN: resource.URN("urn:pulumi:dev:myProj:"),
						Outputs: resource.PropertyMap{
							"a": resource.NewNumberProperty(2.5),
							"b": resource.NewObjectProperty(resource.PropertyMap{"c": resource.NewStringProperty("d")}),
							"c": resource.NewArrayProperty([]resource.PropertyValue{
								resource.NewStringProperty("hello"),
							}),
						},
					},
					{
						URN: resource.URN("urn:pulumi:dev:myProj:"),
						Outputs: resource.PropertyMap{
							"d": resource.NewStringProperty("jpop"),
							"e": resource.NewObjectProperty(
								resource.PropertyMap{
									"j": resource.NewStringProperty("hit"),
									"k": resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("job")}),
								},
							),
							"p": resource.NewArrayProperty([]resource.PropertyValue{resource.NewStringProperty("jello")}),
						},
					},
					{
						URN: resource.URN("urn:pulumi:dev:myProj:"),
						Outputs: resource.PropertyMap{
							"f": resource.NewStringProperty("kay"),
							"g": resource.NewObjectProperty(resource.PropertyMap{
								"a": resource.NewStringProperty("jok"),
								"b": resource.NewArrayProperty([]resource.PropertyValue{
									resource.NewStringProperty("pass"),
									resource.NewObjectProperty(resource.PropertyMap{"A": resource.NewStringProperty("hello")}),
								}),
							}),
							"h": resource.NewArrayProperty([]resource.PropertyValue{
								resource.NewStringProperty("doo"),

								resource.NewObjectProperty(resource.PropertyMap{
									"a": resource.NewStringProperty("crawl"),
									"b": resource.NewArrayProperty([]resource.PropertyValue{
										resource.NewStringProperty("third"),
									}),
								}),
							}),
						},
					},
				},
			}, nil
		},
	}

	cmdBackend.BackendInstance = &backend.MockBackend{
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return &ms, nil
		},
	}

	mws := pkgWs.MockContext{
		ReadProjectF: func() (*workspace.Project, string, error) {
			return &workspace.Project{
				Name: "test-ws",
			}, "", nil
		},
	}
	pkgWs.Instance = &mws

	tests := []struct {
		name string
		args []string
	}{
		{name: "ShowCmdWithoutArgs"},
		{
			name: "ShowCmdWithKeysOnlyFlag",
			args: []string{"--keys-only"},
		},
	}

	ss, err := ms.Snapshot(context.TODO(), &secrets.MockProvider{})
	require.NoError(t, err)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			var cmdOut bytes.Buffer
			showCmd := NewShowCmd()
			showCmd.SetArgs(tst.args)
			showCmd.SetOut(&cmdOut)
			require.NoError(t, showCmd.Execute())
			var CmdPrintopts printOptions
			if flag := showCmd.Flags().Lookup("keys-only"); flag != nil {
				CmdPrintopts.keysOnly = true
			}
			var expectedOut string
			for _, res := range ss.Resources {
				expectedOut += renderResourceState(res, CmdPrintopts) + "\n"
			}
			require.Equal(t, cmdOut.String(), expectedOut)
		})
	}
}
