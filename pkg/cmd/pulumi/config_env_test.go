// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"io"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func cleanStdout(s string) string {
	return strings.ReplaceAll(stripansi.Strip(s), "\r", "")
}

func newConfigEnvCmdForTest(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
	projectYAML string,
	projectStackYAML string,
	env *esc.Environment,
	diags apitype.EnvironmentDiagnostics,
	newStackYAML *string,
) *configEnvCmd {
	stackRef := "stack"
	return &configEnvCmd{
		ctx:         ctx,
		stdin:       stdin,
		stdout:      stdout,
		interactive: true,

		readProject: func() (*workspace.Project, string, error) {
			p, err := workspace.LoadProjectBytes([]byte(projectYAML), "Pulumi.yaml", encoding.YAML)
			if err != nil {
				return nil, "", err
			}
			return p, "", nil
		},
		requireStack: func(
			ctx context.Context,
			stackName string,
			lopt stackLoadOption,
			opts display.Options,
		) (backend.Stack, error) {
			return &backend.MockStack{
				RefF: func() backend.StackReference {
					return &backend.MockStackReference{
						StringV:             "org/stack",
						NameV:               tokens.MustParseStackName("stack"),
						ProjectV:            "project",
						FullyQualifiedNameV: "org/stack",
					}
				},
				OrgNameF: func() string {
					return "org"
				},
				BackendF: func() backend.Backend {
					return &backend.MockEnvironmentsBackend{
						CheckYAMLEnvironmentF: func(
							ctx context.Context,
							org string,
							yaml []byte,
						) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
							return env, diags, nil
						},
					}
				},
				DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
					return b64.NewBase64SecretsManager(), nil
				},
			}, nil
		},
		loadProjectStack: func(p *workspace.Project, _ backend.Stack) (*workspace.ProjectStack, error) {
			return workspace.LoadProjectStackBytes(p, []byte(projectStackYAML), "Pulumi.stack.yaml", encoding.YAML)
		},
		saveProjectStack: func(_ backend.Stack, ps *workspace.ProjectStack) error {
			b, err := encoding.YAML.Marshal(ps)
			if err != nil {
				return err
			}
			*newStackYAML = string(b)
			return nil
		},

		stackRef: &stackRef,
	}
}
