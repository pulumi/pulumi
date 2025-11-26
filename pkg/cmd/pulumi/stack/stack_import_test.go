// Copyright 2025-2025, Pulumi Corporation.
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

package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression test for https://github.com/pulumi/pulumi/issues/20956. Ensure that if we import a deployment
// file that's using the "service" secrets manager that we reconfigure it for the target stack, rather than
// just reusing the existing configuration which might be pointing to a different stack.
func TestStackImport_ChangeServiceSecrets(t *testing.T) {
	t.Parallel()

	settings := &pkgWorkspace.Settings{
		Stack: "org/proj/stk",
	}
	w := &pkgWorkspace.MockW{
		SettingsF: func() *pkgWorkspace.Settings {
			return settings
		},
	}

	newSmState := json.RawMessage(`{"url":"https://api.pulumi.com","owner":"org","project":"proj","stack":"stk"}`)
	newSm := &secrets.MockSecretsManager{
		TypeF: func() string { return "service" },
		StateF: func() json.RawMessage {
			return newSmState
		},
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}

	var be *backend.MockBackend
	stk := &backend.MockStack{
		RefF: func() backend.StackReference {
			return &backend.MockStackReference{
				NameV: tokens.MustParseStackName("stk"),
			}
		},
		BackendF: func() backend.Backend { return be },
		DefaultSecretManagerF: func(info *workspace.ProjectStack) (secrets.Manager, error) {
			return newSm, nil
		},
	}
	be = &backend.MockBackend{
		GetStackF: func(ctx context.Context, ref backend.StackReference) (backend.Stack, error) {
			assert.Equal(t, "org/proj/stk", ref.String())
			return stk, nil
		},
		ImportDeploymentF: func(ctx context.Context, s backend.Stack, deployment *apitype.UntypedDeployment) error {
			// Assert that the secrets manager being used for the import is the new one created for the target stack
			v3deployment, err := stack.UnmarshalUntypedDeployment(ctx, deployment)
			if err != nil {
				return err
			}
			assert.Equal(t, "service", v3deployment.SecretsProviders.Type)
			assert.Equal(t, newSmState, v3deployment.SecretsProviders.State)
			return nil
		},
	}
	ws := &pkgWorkspace.MockContext{
		NewF: func() (pkgWorkspace.W, error) {
			return w, nil
		},
	}
	lm := &cmdBackend.MockLoginManager{
		LoginF: func(ctx context.Context, ws pkgWorkspace.Context, sink diag.Sink, url string,
			project *workspace.Project, setCurrent, insecure bool, color colors.Colorization,
		) (backend.Backend, error) {
			return be, nil
		},
	}
	sp := &secrets.MockProvider{}
	var smState json.RawMessage
	sm := &secrets.MockSecretsManager{
		TypeF: func() string { return "service" },
		StateF: func() json.RawMessage {
			return smState
		},
		DecrypterF: func() config.Decrypter {
			return config.NopDecrypter
		},
		EncrypterF: func() config.Encrypter {
			return config.NopEncrypter
		},
	}
	sp = sp.Add("service", func(state json.RawMessage) (secrets.Manager, error) {
		smState = state
		return sm, nil
	})

	cmd := newStackImportCmd(ws, lm, sp)

	var stdinBuf bytes.Buffer
	importDeployment := `{
	"version": 3,
	"deployment": {
		"secrets_providers": {
			"type": "service",
			"state": {
				"url": "https://api.pulumi.com",
				"owner": "src-org",
				"project": "proj",
				"stack": "stk"
			}
		}
	}
}`
	stdinBuf.WriteString(importDeployment)

	var stdoutBuf bytes.Buffer
	cmd.SetOut(&stdoutBuf)
	cmd.SetIn(&stdinBuf)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
}
