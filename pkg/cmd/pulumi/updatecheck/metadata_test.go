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

package updatecheck

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmdDo "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/do"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func TestGetCLIMetadata(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name     string
		cmd      *cobra.Command
		environ  []string
		args     []string
		metadata map[string]string
	}{
		{
			name:     "nil",
			cmd:      nil,
			metadata: nil,
			environ:  nil,
		},
		{
			name: "no set flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "no-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")
				return cmd
			}(),
			environ: []string{},
			metadata: map[string]string{
				"Command":     "no-set",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "one set bool flag",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "one-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--bool"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			}(),
			metadata: map[string]string{
				"Command":     "one-set",
				"Flags":       "--bool",
				"Environment": "",
			},
		},
		{
			name: "one set string flag",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "one-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--string=value"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			}(),
			metadata: map[string]string{
				"Command":     "one-set",
				"Flags":       "--string",
				"Environment": "",
			},
		},
		{
			name: "multiple set flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "multiple-set"}
				cmd.Flags().Bool("bool", false, "bool flag")
				cmd.Flags().String("string", "", "string flag")

				cmd.SetArgs([]string{"--string=value", "--bool"})

				err := cmd.Execute()
				require.NoError(t, err)

				return cmd
			}(),
			metadata: map[string]string{
				"Command":     "multiple-set",
				"Flags":       "--bool --string",
				"Environment": "",
			},
		},
		{
			name: "longer command path",
			cmd: func() *cobra.Command {
				parent := &cobra.Command{Use: "parent"}
				err := parent.Execute()
				require.NoError(t, err)

				cmd := &cobra.Command{Use: "multiple-set"}
				parent.AddCommand(cmd)

				return cmd
			}(),
			metadata: map[string]string{
				"Command":     "parent multiple-set",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "no valid PULUMI_ env variables",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "version"}
				err := cmd.Execute()
				require.NoError(t, err)
				return cmd
			}(),
			environ: []string{"PULUMICOPILOT=true", "OTHER_FLAG=true", "PULUMI_NO_EQUALS_SIGN"},
			metadata: map[string]string{
				"Command":     "version",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "has valid PULUMI_ env variables",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "version"}
				err := cmd.Execute()
				require.NoError(t, err)
				return cmd
			}(),
			environ: []string{"PULUMI_EXPERIMENTAL=true", "PULUMI_COPILOT=true"},
			metadata: map[string]string{
				"Command":     "version",
				"Flags":       "",
				"Environment": "PULUMI_EXPERIMENTAL PULUMI_COPILOT",
			},
		},
		{
			name: "do with token and operation",
			cmd:  newDoTestCmd(),
			args: []string{"aws:s3:Bucket", "list"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket list",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with flags mixed in",
			cmd:  newDoTestCmd(),
			args: []string{"--dry-run", "aws:s3:Bucket", "create", "--some-unknown-flag", "secret-value"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket create",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with package flag",
			cmd:  newDoTestCmd(),
			args: []string{"--package", "aws", "aws:s3:Bucket", "list"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket list",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do drops extra positionals after verb",
			cmd:  newDoTestCmd(),
			args: []string{"aws:s3:Bucket", "read", "some-resource-id"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:s3:Bucket read",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do drops unknown verb",
			cmd:  newDoTestCmd(),
			args: []string{"aws:lambda:Function", "someUnknownVerb"},
			metadata: map[string]string{
				"Command":     "pulumi do aws:lambda:Function",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "do with no args",
			cmd:  newDoTestCmd(),
			args: []string{},
			metadata: map[string]string{
				"Command":     "pulumi do",
				"Flags":       "",
				"Environment": "",
			},
		},
		{
			name: "plugin run with argument",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{Use: "pulumi"}
				pluginCmd := &cobra.Command{Use: "plugin"}
				cmd.AddCommand(pluginCmd)
				pluginRunCmd := &cobra.Command{Use: "run", Args: cmdutil.MinimumNArgs(1)}
				pluginCmd.AddCommand(pluginRunCmd)
				err := pluginRunCmd.Execute()
				require.NoError(t, err)
				return pluginRunCmd
			}(),
			environ: []string{"PULUMI_EXPERIMENTAL=true", "PULUMI_COPILOT=true"},
			args:    []string{"my-plugin"},
			metadata: map[string]string{
				"Command":     "pulumi plugin run my-plugin",
				"Flags":       "",
				"Environment": "PULUMI_EXPERIMENTAL PULUMI_COPILOT",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Act.
			metadata := GetCLIMetadata(c.cmd, c.environ, c.args)

			// Assert.
			require.Equal(t, c.metadata, metadata)
		})
	}
}

func newDoTestCmd() *cobra.Command {
	root := &cobra.Command{Use: "pulumi"}
	doCmd := cmdDo.NewDoCmd(nil, nil, nil, nil, nil, nil)
	root.AddCommand(doCmd)
	return doCmd
}

func TestPulumiEnvNames(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", pulumiEnvNames(nil))
	assert.Equal(t,
		"PULUMI_HOME PULUMI_SKIP_UPDATE_CHECK PULUMI_CONFIG",
		pulumiEnvNames([]string{
			"PATH=/usr/bin",
			"PULUMI_HOME=/home/user/.pulumi",
			"PULUMIX=nope",
			"PULUMI_SKIP_UPDATE_CHECK=true",
			"PULUMI_NO_EQUALS_SIGN",
			`PULUMI_CONFIG={"key":"a=b"}`,
		}),
	)
}
