// Copyright 2016-2018, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests the output of 'pulumi stack output'
// under different conditions.
func TestStackOutputCmd_plainText(t *testing.T) {
	t.Parallel()

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Any additional command line arguments.
		args []string

		// Expectations from stdout:
		contains    []string
		notContains []string
		equals      string // only valid if non-empty
	}{
		{
			desc:        "default",
			outputs:     outputsWithSecret,
			contains:    []string{"mybucket-1234", "password", "[secret]"},
			notContains: []string{"hunter2"},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			contains:    []string{"mybucket-1234", "password", "hunter2"},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			equals:  "mybucket-1234\n",
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			equals:  "[secret]\n",
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			equals:      "hunter2\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
				string, stackLoadOption, display.Options,
			) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			var stdoutBuff bytes.Buffer
			cmd := stackOutputCmd{
				Stdout:       &stdoutBuff,
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))
			stdout := stdoutBuff.String()

			if tt.equals != "" {
				assert.Equal(t, tt.equals, stdout)
			}
			for _, s := range tt.contains {
				assert.Contains(t, stdout, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, stdout, s)
			}
		})
	}
}

// Tests the output of 'pulumi stack output --json'
// under different conditions.
func TestStackOutputCmd_json(t *testing.T) {
	t.Parallel()

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Any additional command line arguments.
		args []string

		// Expected parsed JSON output.
		want interface{}
	}{
		{
			desc:    "default",
			outputs: outputsWithSecret,
			want: map[string]interface{}{
				"bucketName": "mybucket-1234",
				"password":   "[secret]",
			},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			want: map[string]interface{}{
				"bucketName": "mybucket-1234",
				"password":   "hunter2",
			},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			want:    "mybucket-1234",
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			want:    "[secret]",
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			want:        "hunter2",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
				string, stackLoadOption, display.Options,
			) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			var stdoutBuff bytes.Buffer
			cmd := stackOutputCmd{
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
				jsonOut:      true,
				Stdout:       &stdoutBuff,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))

			stdout := stdoutBuff.Bytes()
			var got interface{}
			require.NoError(t, json.Unmarshal(stdout, &got),
				"output is not valid JSON:\n%s", stdout)

			assert.Equal(t, tt.want, got)
		})
	}
}

// Tests the output of 'pulumi stack output --shell'
// under different conditions.
func TestStackOutputCmd_shell(t *testing.T) {
	t.Parallel()

	outputsWithSecret := resource.PropertyMap{
		"bucketName": resource.NewStringProperty("mybucket-1234"),
		"password": resource.NewSecretProperty(&resource.Secret{
			Element: resource.NewStringProperty("hunter2"),
		}),
	}

	tests := []struct {
		desc string

		// Map of stack outputs.
		outputs resource.PropertyMap

		// Whether the --show-secrets flag is set.
		showSecrets bool

		// Current operating system.
		// Defaults to "linux".
		os string

		// Any additional command line arguments.
		args []string

		// Lines expected in the output.
		want []string
	}{
		{
			desc:    "default",
			outputs: outputsWithSecret,
			want: []string{
				"bucketName=mybucket-1234",
				`password=\[secret]`,
			},
		},
		{
			desc:        "show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			want: []string{
				"bucketName=mybucket-1234",
				"password=hunter2",
			},
		},
		{
			desc:    "single property",
			outputs: outputsWithSecret,
			args:    []string{"bucketName"},
			want:    []string{"bucketName=mybucket-1234"},
		},
		{
			// Should not show the secret even if requested
			// if --show-secrets is not set.
			desc:    "single hidden property",
			outputs: outputsWithSecret,
			args:    []string{"password"},
			want:    []string{`password=\[secret]`},
		},
		{
			desc:        "single property with show-secrets",
			outputs:     outputsWithSecret,
			showSecrets: true,
			args:        []string{"password"},
			want:        []string{"password=hunter2"},
		},
		{
			desc:    "powershell on windows",
			outputs: outputsWithSecret,
			os:      "windows",
			want: []string{
				"$bucketName = 'mybucket-1234'",
				"$password = '[secret]'",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			snap := deploy.Snapshot{
				Resources: []*resource.State{
					{
						Type:    resource.RootStackType,
						Outputs: tt.outputs,
					},
				},
			}
			requireStack := func(context.Context, pkgWorkspace.Context, cmdBackend.LoginManager,
				string, stackLoadOption, display.Options,
			) (backend.Stack, error) {
				return &backend.MockStack{
					SnapshotF: func(_ context.Context, _ secrets.Provider) (*deploy.Snapshot, error) {
						return &snap, nil
					},
				}, nil
			}

			osys := tt.os
			if len(osys) == 0 {
				osys = "linux"
			}

			var stdoutBuff bytes.Buffer
			cmd := stackOutputCmd{
				requireStack: requireStack,
				showSecrets:  tt.showSecrets,
				shellOut:     true,
				OS:           osys,
				Stdout:       &stdoutBuff,
			}
			require.NoError(t, cmd.Run(context.Background(), tt.args))

			// Drop trailing "\n" from stdout
			// rather than add a "" at the end of every tt.want.
			stdout := strings.TrimSuffix(stdoutBuff.String(), "\n")
			got := strings.Split(stdout, "\n")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStackOutputCmd_jsonAndShellConflict(t *testing.T) {
	t.Parallel()

	cmd := stackOutputCmd{
		requireStack: func(
			context.Context, pkgWorkspace.Context, cmdBackend.LoginManager, string, stackLoadOption, display.Options,
		) (backend.Stack, error) {
			t.Fatal("This function should not be called")
			return nil, errors.New("should not be called")
		},
		shellOut: true,
		jsonOut:  true,
	}

	err := cmd.Run(context.Background(), nil)
	assert.ErrorContains(t, err, "only one of --json and --shell may be set")
}

func TestShellStackOutputWriter_quoting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc     string
		give     interface{}
		wantBash string
		wantPwsh string
	}{
		{
			desc:     "string",
			give:     "foo",
			wantBash: "foo",
			wantPwsh: "'foo'",
		},
		{
			desc:     "number",
			give:     42,
			wantBash: "42",
			wantPwsh: "'42'",
		},
		{
			desc:     "string/spaces",
			give:     "foo bar",
			wantBash: "'foo bar'",
			wantPwsh: "'foo bar'",
		},
		{
			desc:     "string/double quotes",
			give:     `foo "bar" baz`,
			wantBash: `'foo "bar" baz'`,
			wantPwsh: `'foo "bar" baz'`,
		},
		{
			desc:     "string/single quotes",
			give:     "foo 'bar' baz",
			wantBash: `'foo '\''bar'\'' baz'`,
			wantPwsh: `'foo ''bar'' baz'`,
		},
		{
			desc:     "string/single and double quotes",
			give:     `foo "bar" 'baz' qux`,
			wantBash: `'foo "bar" '\''baz'\'' qux'`,
			wantPwsh: `'foo "bar" ''baz'' qux'`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			t.Run("bash", func(t *testing.T) {
				var got bytes.Buffer
				writer := bashStackOutputWriter{W: &got}
				require.NoError(t, writer.WriteOne("myoutput", tt.give))

				want := "myoutput=" + tt.wantBash + "\n"
				assert.Equal(t, want, got.String())
			})

			t.Run("pwsh", func(t *testing.T) {
				var got bytes.Buffer
				writer := powershellStackOutputWriter{W: &got}
				require.NoError(t, writer.WriteOne("myoutput", tt.give))

				want := "$myoutput = " + tt.wantPwsh + "\n"
				assert.Equal(t, want, got.String())
			})
		})
	}
}
