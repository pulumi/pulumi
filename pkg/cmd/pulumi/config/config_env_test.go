// Copyright 2016-2024, Pulumi Corporation.
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

package config

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newConfigEnvCmdForTest(
	stdin io.Reader,
	stdout io.Writer,
	projectYAML string,
	projectStackYAML string,
	env *esc.Environment,
	diags apitype.EnvironmentDiagnostics,
	newStackYAML *string,
) *configEnvCmd {
	return newConfigEnvCmdForTestWithCheckYAMLEnvironment(
		stdin,
		stdout,
		projectYAML,
		projectStackYAML,
		nil,
		func(
			ctx context.Context,
			org string,
			yaml []byte,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			return env, diags, nil
		},
		newStackYAML,
	)
}

func newConfigEnvCmdForTestWithCheckYAMLEnvironment(
	stdin io.Reader,
	stdout io.Writer,
	projectYAML string,
	projectStackYAML string,
	createEnvironment func(
		ctx context.Context,
		org string,
		project string,
		name string,
		yaml []byte,
	) (apitype.EnvironmentDiagnostics, error),
	checkYAMLEnvironment func(
		ctx context.Context,
		org string,
		yaml []byte,
	) (*esc.Environment, apitype.EnvironmentDiagnostics, error),
	newStackYAML *string,
) *configEnvCmd {
	stackRef := "stack"
	return &configEnvCmd{
		stdin:       stdin,
		stdout:      stdout,
		interactive: true,

		ws: &pkgWorkspace.MockContext{
			ReadProjectF: func() (*workspace.Project, string, error) {
				p, err := workspace.LoadProjectBytes([]byte(projectYAML), "Pulumi.yaml", encoding.YAML)
				if err != nil {
					return nil, "", err
				}
				return p, "", nil
			},
		},
		requireStack: func(
			ctx context.Context,
			ws pkgWorkspace.Context,
			lm cmdBackend.LoginManager,
			stackName string,
			lopt cmdStack.LoadOption,
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
						CreateEnvironmentF:    createEnvironment,
						CheckYAMLEnvironmentF: checkYAMLEnvironment,
					}
				},
				DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
					return b64.NewBase64SecretsManager(), nil
				},
				LoadF: func(ctx context.Context, project *workspace.Project, configFileOverride string,
				) (*workspace.ProjectStack, error) {
					return workspace.LoadProjectStackBytes(project, []byte(projectStackYAML), "Pulumi.stack.yaml", encoding.YAML)
				},
				SaveF: func(ctx context.Context, project *workspace.ProjectStack, configFileOverride string) error {
					yaml, err := encoding.YAML.Marshal(project)
					if err != nil {
						return err
					}
					if newStackYAML != nil {
						*newStackYAML = string(yaml)
					}
					return nil
				},
			}, nil
		},

		stackRef: &stackRef,
	}
}

func mapEvalDiags(diags syntax.Diagnostics) apitype.EnvironmentDiagnostics {
	if len(diags) == 0 {
		return nil
	}

	api := make(apitype.EnvironmentDiagnostics, len(diags))
	for i, d := range diags {
		var rng *esc.Range
		if d.Subject != nil {
			begin := esc.Pos{
				Line:   d.Subject.Start.Line,
				Column: d.Subject.Start.Column,
				Byte:   d.Subject.Start.Byte,
			}
			end := esc.Pos{
				Line:   d.Subject.End.Line,
				Column: d.Subject.End.Column,
				Byte:   d.Subject.End.Byte,
			}
			rng = &esc.Range{
				Environment: d.Subject.Filename,
				Begin:       begin,
				End:         end,
			}
		}
		api[i] = apitype.EnvironmentDiagnostic{
			Range:   rng,
			Summary: d.Summary,
		}
	}

	return api
}

type envDefMap map[string]string

// LoadEnvironment loads the definition for the environment with the given name.
func (m envDefMap) LoadEnvironment(ctx context.Context, name string) ([]byte, eval.Decrypter, error) {
	def, ok := m[name]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	return []byte(def), nil, nil
}

func newConfigEnvCmdForInitTest(
	stdin io.Reader,
	stdout io.Writer,
	projectYAML string,
	projectStackYAML string,
	newStackYAML *string,
	envs envDefMap,
) *configEnvCmd {
	return newConfigEnvCmdForTestWithCheckYAMLEnvironment(
		stdin,
		stdout,
		projectYAML,
		projectStackYAML,
		func(
			ctx context.Context,
			org string,
			project string,
			name string,
			yaml []byte,
		) (apitype.EnvironmentDiagnostics, error) {
			decl, diags, err := eval.LoadYAMLBytes(name, yaml)
			if err != nil {
				return nil, err
			}
			_, checkDiags := eval.CheckEnvironment(ctx, name, decl, nil, nil, envs, &esc.ExecContext{}, false)
			diags.Extend(checkDiags...)
			if len(diags) != 0 {
				return mapEvalDiags(diags), nil
			}
			envs[name] = string(yaml)
			return nil, nil
		},
		func(
			ctx context.Context,
			org string,
			yaml []byte,
		) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
			decl, diags, err := eval.LoadYAMLBytes("<yaml>", yaml)
			if err != nil {
				return nil, nil, err
			}
			env, checkDiags := eval.CheckEnvironment(ctx, "<yaml>", decl, nil, nil, envs, &esc.ExecContext{}, false)
			diags.Extend(checkDiags...)
			return env, mapEvalDiags(diags), nil
		},
		newStackYAML,
	)
}

// The library sending the confirmation prompt may be able to print the prompt
// in full before recognizing the character we send to stdin for the test.
// There's nothing really wrong with that other than it makes the tests flake.
// This cleans the extra output from stdout in case it happens, as it either
// happening or not happening is fine.
func cleanStdoutIncludingPrompt(stdout string) string {
	return strings.ReplaceAll(strings.ReplaceAll(stripansi.Strip(stdout), "\r", ""), "Save? ▸Yes  No", "")
}
