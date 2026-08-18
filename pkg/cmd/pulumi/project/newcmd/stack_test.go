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

package newcmd

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
)

// stackCreationBackend fails CreateStack a set number of times, then keeps failing or
// succeeding per the remaining count, recording each attempted name.
func stackCreationBackend(t *testing.T, failures int, created *[]string) *backend.MockBackend {
	return &backend.MockBackend{
		SupportsOrganizationsF: func() bool { return false },
		GetDefaultOrgF:         func(ctx context.Context) (string, error) { return "", nil },
		ValidateStackNameF:     func(s string) error { return nil },
		ParseStackReferenceF: func(s string) (backend.StackReference, error) {
			return &backend.MockStackReference{
				NameV:   tokens.MustParseStackName(s),
				StringV: s,
			}, nil
		},
		DefaultSecretManagerF: func(ctx context.Context, ps *workspace.ProjectStack) (secrets.Manager, error) {
			return nil, nil
		},
		CreateStackF: func(ctx context.Context, ref backend.StackReference, root string,
			initialState *apitype.UntypedDeployment, opts *backend.CreateStackOptions,
		) (backend.Stack, error) {
			*created = append(*created, ref.String())
			if len(*created) <= failures {
				return nil, errors.New("stack exists")
			}
			return &backend.MockStack{RefF: func() backend.StackReference { return ref }}, nil
		},
	}
}

//nolint:paralleltest // uses process stdout
func TestPromptAndCreateStackNamedStackHardErrors(t *testing.T) {
	var created []string
	b := stackCreationBackend(t, 1, &created)
	prompts := 0
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		prompts++
		return defaultValue, nil
	}

	_, err := PromptAndCreateStack(t.Context(), cmdutil.Diag(), pkgWorkspace.Instance,
		b, prompt, "named", t.TempDir(), false, false, display.Options{}, "default", false, "")

	assert.Error(t, err, "a pre-decided stack name must not retry")
	assert.Equal(t, 0, prompts, "a pre-decided stack name must not prompt")
	assert.Equal(t, []string{"named"}, created)
}

func TestCreateStackWithRetryRepromptsOnFailure(t *testing.T) {
	t.Parallel()

	var created []string
	b := stackCreationBackend(t, 1, &created)
	names := []string{"second"}
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		require.NotEmpty(t, names, "unexpected extra prompt")
		name := names[0]
		names = names[1:]
		return name, nil
	}

	s, resolved, err := createStackWithRetry(t.Context(), cmdutil.Diag(), io.Discard, pkgWorkspace.Instance,
		b, prompt, "first", t.TempDir(), false, display.Options{},
		cmdStack.CreateStackOptions{SecretsProvider: "default", Quiet: true})

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, created)
	assert.Equal(t, "second", resolved)
	require.NotNil(t, s)
}

func TestCreateStackWithRetryDefaultOrgFailureIsFatal(t *testing.T) {
	t.Parallel()

	var created []string
	b := stackCreationBackend(t, 1, &created)
	b.GetDefaultOrgF = func(ctx context.Context) (string, error) {
		return "", errors.New("could not determine default org")
	}
	prompt := func(yes bool, valueType, defaultValue string, secret bool,
		isValidFn func(string) error, opts display.Options,
	) (string, error) {
		t.Fatal("unexpected prompt: a buildStackName failure must not retry")
		return "", nil
	}

	s, resolved, err := createStackWithRetry(t.Context(), cmdutil.Diag(), io.Discard, pkgWorkspace.Instance,
		b, prompt, "first", t.TempDir(), false, display.Options{},
		cmdStack.CreateStackOptions{SecretsProvider: "default", Quiet: true})

	assert.Error(t, err, "a buildStackName failure must be fatal")
	assert.Empty(t, created, "CreateStack must not be attempted when buildStackName fails")
	assert.Empty(t, resolved)
	assert.Nil(t, s)
}
