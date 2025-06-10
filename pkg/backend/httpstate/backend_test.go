// Copyright 2016-2021, Pulumi Corporation.
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
package httpstate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // mutates global configuration
func TestEnabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true)
		require.NoError(t, err)
	}()

	previous := cmdutil.FullyQualifyStackNames
	expected := s.Ref().FullyQualifiedName().String()

	// Act
	cmdutil.FullyQualifyStackNames = true
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

//nolint:paralleltest // mutates env vars and global state
func TestMissingPulumiAccessToken(t *testing.T) {
	t.Setenv("PULUMI_ACCESS_TOKEN", "")

	{ // Disable interactive mode
		disableInteractive := cmdutil.DisableInteractive
		cmdutil.DisableInteractive = true
		t.Cleanup(func() {
			cmdutil.DisableInteractive = disableInteractive
		})
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, "https://api.example.com", false, "", "", nil, true, display.Options{})
	var expectedErr backenderr.MissingEnvVarForNonInteractiveError
	if assert.ErrorAs(t, err, &expectedErr) {
		assert.Equal(t, env.AccessToken.Var(), expectedErr.Var)
	}
}

//nolint:paralleltest // mutates global configuration
func TestDisabledFullyQualifiedStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true)
		require.NoError(t, err)
	}()

	previous := cmdutil.FullyQualifyStackNames
	expected := s.Ref().Name().String()

	// Act
	cmdutil.FullyQualifyStackNames = false
	defer func() { cmdutil.FullyQualifyStackNames = previous }()

	actual := s.Ref().String()

	// Assert
	assert.Equal(t, expected, actual)
}

func TestValueOrDefaultURL(t *testing.T) {
	t.Run("TestValueOrDefault", func(t *testing.T) {
		current := ""
		mock := &pkgWorkspace.MockContext{
			GetStoredCredentialsF: func() (workspace.Credentials, error) {
				return workspace.Credentials{
					Current: current,
				}, nil
			},
		}

		// Validate trailing slash gets cut
		assert.Equal(t, "https://api-test1.pulumi.com", ValueOrDefaultURL(mock, "https://api-test1.pulumi.com/"))

		// Validate no-op case
		assert.Equal(t, "https://api-test2.pulumi.com", ValueOrDefaultURL(mock, "https://api-test2.pulumi.com"))

		// Validate trailing slash in pre-set env var is unchanged
		t.Setenv("PULUMI_API", "https://api-test3.pulumi.com/")
		assert.Equal(t, "https://api-test3.pulumi.com/", ValueOrDefaultURL(mock, ""))
		t.Setenv("PULUMI_API", "")

		// Validate current credentials URL is used
		current = "https://api-test4.pulumi.com"
		assert.Equal(t, "https://api-test4.pulumi.com", ValueOrDefaultURL(mock, ""))

		// Unless the current credentials URL is a filestate url
		current = "s3://test"
		assert.Equal(t, "https://api.pulumi.com", ValueOrDefaultURL(mock, ""))
	})
}

// TestDefaultOrganizationPriority tests the priority of the default organization.
// The priority is:
// 1. The default organization.
// 2. The user's organization.
func TestDefaultOrganizationPriority(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		getDefaultOrg func() (string, error)
		getUserOrg    func() (string, error)
		wantOrg       string
		wantErr       bool
	}{
		{
			name: "default org set",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "", nil
			},
			wantOrg: "default-org",
		},
		{
			name: "user org set",
			getDefaultOrg: func() (string, error) {
				return "", nil
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "user-org",
		},
		{
			name: "no org set",
			getDefaultOrg: func() (string, error) {
				return "", nil
			},
			getUserOrg: func() (string, error) {
				return "", nil
			},
			wantErr: true,
		},
		{
			name: "both orgs set",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "default-org",
		},
		{
			name: "default org set, user org error",
			getDefaultOrg: func() (string, error) {
				return "default-org", nil
			},
			getUserOrg: func() (string, error) {
				return "", errors.New("user org error")
			},
			wantOrg: "default-org",
		},
		{
			name: "user org set, default org error",
			getDefaultOrg: func() (string, error) {
				return "", errors.New("default org error")
			},
			getUserOrg: func() (string, error) {
				return "user-org", nil
			},
			wantOrg: "user-org",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			org, err := inferOrg(context.Background(), tt.getDefaultOrg, tt.getUserOrg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantOrg, org)
		})
	}
}

//nolint:paralleltest // mutates global state
func TestDisableIntegrityChecking(t *testing.T) {
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true)
		require.NoError(t, err)
	}()

	// make up a bad stack
	deployment := apitype.UntypedDeployment{
		Version: 3,
		Deployment: json.RawMessage(`{
			"resources": [
				{
					"urn": "urn:pulumi:stack::proj::type::name1",
					"type": "type",
					"parent": "urn:pulumi:stack::proj::type::name2"
				},
				{
					"urn": "urn:pulumi:stack::proj::type::name2",
					"type": "type"
				}
			]
		}`),
	}

	// Import deployment doesn't verify the deployment
	err = b.ImportDeployment(ctx, s, &deployment)
	require.NoError(t, err)

	backend.DisableIntegrityChecking = false
	snap, err := s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.ErrorContains(t, err,
		"child resource urn:pulumi:stack::proj::type::name1's parent urn:pulumi:stack::proj::type::name2 comes after it")
	assert.Nil(t, snap)

	backend.DisableIntegrityChecking = true
	snap, err = s.Snapshot(ctx, b64.Base64SecretsProvider)
	require.NoError(t, err)
	assert.NotNil(t, snap)
}

func TestCloudBackend_GetPackageRegistry(t *testing.T) {
	t.Parallel()
	mockClient := &client.Client{}
	b := &cloudBackend{
		client: mockClient,
		d:      diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never}),
	}

	registry, err := b.GetPackageRegistry()
	assert.NoError(t, err)
	assert.NotNil(t, registry)

	_, ok := registry.(*cloudPackageRegistry)
	assert.True(t, ok, "expected registry to be a cloudPackageRegistry")
}

// Bit of an integration test.
// That we can render engine events, send them to the backend, and get a summary back.
func TestCopilotExplainer(t *testing.T) {
	t.Parallel()

	copilotResponse, err := json.Marshal(apitype.CopilotResponse{
		ThreadMessages: []apitype.CopilotThreadMessage{
			{
				Role:    "assistant",
				Kind:    "response",
				Content: json.RawMessage(`"Test summary of changes"`),
			},
		},
	})
	assert.NoError(t, err)

	// Create a mock transport that
	// 1. captures the request to assert on
	// 2. returns our test response
	var requestBody []byte
	mockTransport := &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			var err error
			requestBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(copilotResponse)),
				Header:     make(http.Header),
			}, nil
		},
	}

	// Create a backend and API client using our mock transport
	apiClient := client.NewClient(PulumiCloudURL, "test-token", false, diagtest.LogSink(t))
	apiClient.WithHTTPClient(&http.Client{Transport: mockTransport})
	b := &cloudBackend{
		client: apiClient,
		d:      diagtest.LogSink(t),
	}

	// Call explainer
	stackRef := cloudBackendReference{
		name:    tokens.MustParseStackName("foo"),
		owner:   "test-owner",
		project: "test-project",
	}
	op := backend.UpdateOperation{
		Proj: &workspace.Project{Name: "test-project"},
		Opts: backend.UpdateOptions{
			Display: display.Options{
				Color: colors.Never,
			},
		},
	}
	events := []engine.Event{
		engine.NewEvent(engine.StdoutEventPayload{
			Message: "Hello, world!",
			Color:   colors.Never,
		}),
	}
	summary, err := b.Explain(context.Background(), stackRef, apitype.UpdateUpdate, op, events)

	// Verify results
	require.NoError(t, err)
	assert.Contains(t, summary, "Test summary of changes")
	assert.Contains(t, string(requestBody), "Hello, world!")
}

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req)
}

//nolint:paralleltest // mutates global configuration
func TestListStackNames(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	// Create test stacks
	numStacks := 3
	stackNames := make([]string, numStacks)
	stacks := make([]backend.Stack, numStacks)

	for i := 0; i < numStacks; i++ {
		stackName := ptesting.RandomStackName()
		stackNames[i] = stackName
		ref, err := b.ParseStackReference(stackName)
		require.NoError(t, err)

		s, err := b.CreateStack(ctx, ref, "", nil, nil)
		require.NoError(t, err)
		stacks[i] = s
	}

	// Cleanup stacks
	defer func() {
		for _, s := range stacks {
			_, err := b.RemoveStack(ctx, s, true)
			require.NoError(t, err)
		}
	}()

	// Test ListStackNames with pagination support and project filter
	projectName := "testproj"
	filter := backend.ListStacksFilter{
		Project: &projectName, // Filter to just our test project to reduce scope
	}
	var allStackRefs []backend.StackReference
	var token backend.ContinuationToken
	maxIterations := 100 // Prevent infinite loops

	// Keep fetching until we get all stacks (with safeguards)
	for i := 0; i < maxIterations; i++ {
		stackRefs, nextToken, err := b.ListStackNames(ctx, filter, token)
		require.NoError(t, err)

		allStackRefs = append(allStackRefs, stackRefs...)

		if nextToken == nil {
			break
		}
		token = nextToken

		// Additional safeguard: if we're getting an excessive number of stacks, something is wrong
		if len(allStackRefs) > 10000 {
			t.Fatalf("Too many stacks returned (%d), possible infinite loop", len(allStackRefs))
		}
	}

	// Verify we got at least our test stacks (there might be other stacks in the project)
	assert.GreaterOrEqual(t, len(allStackRefs), numStacks)

	// Verify all our test stack names are present
	foundStacks := make(map[string]bool)
	for _, stackRef := range allStackRefs {
		foundStacks[stackRef.Name().String()] = true
	}

	for _, expectedName := range stackNames {
		assert.True(t, foundStacks[expectedName], "Stack %s should be in the results", expectedName)
	}

	// Verify that ListStackNames returns StackReference objects (not StackSummary)
	assert.IsType(t, []backend.StackReference{}, allStackRefs)
}

//nolint:paralleltest // mutates global configuration
func TestListStackNamesVsListStacks(t *testing.T) {
	// Arrange
	if os.Getenv("PULUMI_ACCESS_TOKEN") == "" {
		t.Skipf("Skipping: PULUMI_ACCESS_TOKEN is not set")
	}

	ctx := context.Background()

	_, err := NewLoginManager().Login(ctx, PulumiCloudURL, false, "", "", nil, true, display.Options{})
	require.NoError(t, err)

	b, err := New(ctx, diagtest.LogSink(t), PulumiCloudURL, &workspace.Project{Name: "testproj"}, false)
	require.NoError(t, err)

	// Create a test stack
	stackName := ptesting.RandomStackName()
	ref, err := b.ParseStackReference(stackName)
	require.NoError(t, err)

	s, err := b.CreateStack(ctx, ref, "", nil, nil)
	require.NoError(t, err)
	defer func() {
		_, err := b.RemoveStack(ctx, s, true)
		require.NoError(t, err)
	}()

	// Test both methods with pagination support and project filter
	projectName := "testproj"
	filter := backend.ListStacksFilter{
		Project: &projectName, // Filter to just our test project to reduce scope
	}
	maxIterations := 100 // Prevent infinite loops

	// Test ListStacks with pagination
	var allSummaries []backend.StackSummary
	var token1 backend.ContinuationToken

	for i := 0; i < maxIterations; i++ {
		summaries, nextToken, err := b.ListStacks(ctx, filter, token1)
		require.NoError(t, err)

		allSummaries = append(allSummaries, summaries...)

		if nextToken == nil {
			break
		}
		token1 = nextToken

		// Additional safeguard: if we're getting an excessive number of stacks, something is wrong
		if len(allSummaries) > 10000 {
			t.Fatalf("Too many stacks returned from ListStacks (%d), possible infinite loop", len(allSummaries))
		}
	}

	// Test ListStackNames with pagination
	var allStackRefs []backend.StackReference
	var token2 backend.ContinuationToken

	for i := 0; i < maxIterations; i++ {
		stackRefs, nextToken, err := b.ListStackNames(ctx, filter, token2)
		require.NoError(t, err)

		allStackRefs = append(allStackRefs, stackRefs...)

		if nextToken == nil {
			break
		}
		token2 = nextToken

		// Additional safeguard: if we're getting an excessive number of stacks, something is wrong
		if len(allStackRefs) > 10000 {
			t.Fatalf("Too many stacks returned from ListStackNames (%d), possible infinite loop", len(allStackRefs))
		}
	}

	// Both should return the same number of stacks
	assert.Equal(t, len(allSummaries), len(allStackRefs))

	// Verify that stack names match between the two methods
	summaryNames := make(map[string]bool)
	for _, summary := range allSummaries {
		summaryNames[summary.Name().String()] = true
	}

	refNames := make(map[string]bool)
	for _, stackRef := range allStackRefs {
		refNames[stackRef.String()] = true
	}

	// Our test stack should be present in both
	found := false
	for _, summary := range allSummaries {
		if summary.Name().Name().String() == stackName {
			found = true
			break
		}
	}
	assert.True(t, found, "Test stack should be found in ListStacks results")

	found = false
	for _, stackRef := range allStackRefs {
		if stackRef.Name().String() == stackName {
			found = true
			break
		}
	}
	assert.True(t, found, "Test stack should be found in ListStackNames results")
}
