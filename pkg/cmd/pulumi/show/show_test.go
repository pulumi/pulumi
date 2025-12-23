// Copyright 2025, Pulumi Corporation.
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
	"github.com/stretchr/testify/require"
)

func TestShowCmd(t *testing.T) {
	ms := backend.MockStack{
		SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
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
	msName := "test-stack"
	cmdBackend.BackendInstance = &backend.MockBackend{
		GetStackF: func(_ context.Context, _ backend.StackReference) (backend.Stack, error) {
			return &ms, nil
		},
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "ShowCmdWithoutArgs", args: []string{"--stack", msName}},
		{
			name: "ShowCmdWithKeysOnlyFlag",
			args: []string{"--keys-only", "--stack", msName},
		},
	}

	ss, err := ms.Snapshot(context.TODO(), &secrets.MockProvider{})
	require.NoError(t, err)

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			var cmdOut bytes.Buffer
			cmdOpts := ShowCmdOpts{
				Lm: cmdBackend.DefaultLoginManager,
				Sp: &secrets.MockProvider{},
				Ws: pkgWs.Instance,
			}
			showCmd := NewShowCmd(cmdOpts)
			showCmd.SetArgs(tst.args)
			showCmd.SetOut(&cmdOut)
			require.NoError(t, showCmd.Execute())
			var CmdPrintopts printOptions
			keysOnly, err := showCmd.Flags().GetBool("keys-only")
			require.NoError(t, err)
			if keysOnly {
				CmdPrintopts.keysOnly = keysOnly
			}

			var expectedOut bytes.Buffer
			resources := ss.Resources
			resources = resources[1:]
			for _, res := range resources {
				printResourceState(res, CmdPrintopts, &expectedOut)
			}

			require.Equal(t, cmdOut, expectedOut)
		})
	}
}
