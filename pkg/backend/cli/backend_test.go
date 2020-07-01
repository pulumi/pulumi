package cli

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	backend_test "github.com/pulumi/pulumi/pkg/v2/backend/testing"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v2/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v2/util/cancel"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

var defaultProject = &workspace.Project{
	Name: "project",
}

func testStack(owner, project, stack string) *backend_test.Stack {
	return &backend_test.Stack{
		ID: backend.StackIdentifier{
			Owner:   owner,
			Project: project,
			Stack:   stack,
		},
	}
}

func newClient(stacks ...*backend_test.Stack) *backend_test.Client {
	return backend_test.NewClient(backend_test.ClientConfig{
		Name: "test",
		User: "user",
	}, stacks...)
}

func newBackend(client *backend_test.Client, project *workspace.Project) *Backend {
	return &Backend{
		d:              diag.DefaultSink(ioutil.Discard, ioutil.Discard, diag.FormatOptions{Color: colors.Never}),
		currentProject: project,
		client:         client,
	}
}

type cancellationScopeSource int

func (cancellationScopeSource) NewScope(events chan<- engine.Event, isPreview bool) CancellationScope {
	cancelContext, _ := cancel.NewContext(context.Background())
	return &cancellationScope{context: cancelContext}
}

type cancellationScope struct {
	context *cancel.Context
}

func (s *cancellationScope) Context() *cancel.Context {
	return s.context
}

func (s *cancellationScope) Close() {
}

func TestCreateStack(t *testing.T) {
	stackID := backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	}

	client := newClient()
	b := newBackend(client, defaultProject)

	_, err := b.CreateStack(context.Background(), stackID)
	assert.NoError(t, err)

	assert.Len(t, client.Stacks, 1)
}

func TestListStacks(t *testing.T) {
	client := newClient(
		testStack("user", "project", "stack"),
		testStack("user", "project", "stack2"),
		testStack("user", "project2", "stack"))
	b := newBackend(client, defaultProject)

	stacks, err := b.ListStacks(context.Background(), backend.ListStacksFilter{})
	assert.NoError(t, err)
	assert.Len(t, stacks, 3)

	projectName := "project"
	stacks, err = b.ListStacks(context.Background(), backend.ListStacksFilter{Project: &projectName})
	assert.NoError(t, err)
	assert.Len(t, stacks, 2)
}

func TestGetStack(t *testing.T) {
	client := newClient(testStack("user", "project", "stack"))
	b := newBackend(client, defaultProject)

	s, err := b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	s, err = b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack2",
	})
	assert.NoError(t, err)
	assert.Nil(t, s)
}

func TestRemoveStack(t *testing.T) {
	client := newClient(testStack("user", "project", "stack"))
	b := newBackend(client, defaultProject)

	s, err := b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	_, err = b.RemoveStack(context.Background(), s, false)
	assert.NoError(t, err)
	assert.Len(t, client.Stacks, 0)
}

func TestRenameStack(t *testing.T) {
	client := newClient(testStack("user", "project", "stack"))
	b := newBackend(client, defaultProject)

	s, err := b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	newID, err := b.RenameStack(context.Background(), s, "user/project/stack2")
	assert.NoError(t, err)
	assert.Equal(t, backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack2",
	}, newID)

	_, err = b.RenameStack(context.Background(), s, "user2/project/stack2")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match existing owner")
}

func TestPreview(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	client := newClient(testStack("user", "project", "stack"))
	b := newBackend(client, defaultProject)

	s, err := b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	var stdout, stderr bytes.Buffer
	changes, res := s.Preview(context.Background(), UpdateOperation{
		Proj: defaultProject,
		M:    &UpdateMetadata{},
		Opts: UpdateOptions{
			Engine: engine.UpdateOptions{Host: host},
			Display: display.Options{
				Color:  colors.Never,
				Stdout: &stdout,
				Stderr: &stderr,
			},
		},
		SecretsManager: b64.NewBase64SecretsManager(),
		Scopes:         cancellationScopeSource(0),
	})
	assert.Nil(t, res)
	assert.Equal(t, engine.ResourceChanges{
		deploy.OpCreate: 1,
	}, changes)

	stdoutText := stdout.String()
	assert.Contains(t, stdoutText, "+  pkgA:m:typA resA create")
	assert.Contains(t, stdoutText, "pulumi:pulumi:Stack project-stack")
	assert.Contains(t, stdoutText, "+ 1 to create")

	updates, err := b.GetHistory(context.Background(), s.ID())
	assert.NoError(t, err)
	assert.Len(t, updates, 0)
}

func TestUpdate(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	client := newClient(testStack("user", "project", "stack"))
	b := newBackend(client, defaultProject)

	s, err := b.GetStack(context.Background(), backend.StackIdentifier{
		Owner:   "user",
		Project: "project",
		Stack:   "stack",
	})
	assert.NoError(t, err)
	assert.NotNil(t, s)

	var stdout, stderr bytes.Buffer
	changes, res := s.Update(context.Background(), UpdateOperation{
		Proj: defaultProject,
		M:    &UpdateMetadata{},
		Opts: UpdateOptions{
			SkipPreview: true,
			Engine:      engine.UpdateOptions{Host: host},
			Display: display.Options{
				Color:  colors.Never,
				Stdout: &stdout,
				Stderr: &stderr,
			},
		},
		SecretsManager: b64.NewBase64SecretsManager(),
		Scopes:         cancellationScopeSource(0),
	})
	assert.Nil(t, res)
	assert.Equal(t, engine.ResourceChanges{
		deploy.OpCreate: 1,
	}, changes)

	stdoutText := stdout.String()
	assert.Contains(t, stdoutText, "+  pkgA:m:typA resA creating")
	assert.Contains(t, stdoutText, "+  pkgA:m:typA resA created")
	assert.Contains(t, stdoutText, "pulumi:pulumi:Stack project-stack")
	assert.Contains(t, stdoutText, "+ 1 created")

	updates, err := b.GetHistory(context.Background(), s.ID())
	assert.NoError(t, err)
	assert.Len(t, updates, 1)
}
