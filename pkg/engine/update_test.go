package engine

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/util/cancel"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbbreviateFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/Users/username/test-policy",
			expected: "/Users/username/test-policy",
		},
		{
			path:     "./..//test-policy",
			expected: "../test-policy",
		},
		{
			path: `/Users/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "/Users/.../twelve/test-policy",
		},
		{
			path: `nonrootdir/username/averylongpath/one/two/three/four/` +
				`five/six/seven/eight/nine/ten/eleven/twelve/test-policy`,
			expected: "nonrootdir/username/.../twelve/test-policy",
		},
		{
			path: `C:/Documents and Settings/username/My Documents/averylongpath/` +
				`one/two/three/four/five/six/seven/eight/test-policy`,
			expected: "C:/Documents and Settings/.../eight/test-policy",
		},
		{
			path: `C:\Documents and Settings\username\My Documents\averylongpath\` +
				`one\two\three\four\five\six\seven\eight\test-policy`,
			expected: `C:\Documents and Settings\...\eight\test-policy`,
		},
	}

	for _, tt := range tests {
		actual := abbreviateFilePath(tt.path)
		assert.Equal(t, filepath.ToSlash(tt.expected), filepath.ToSlash(actual))
	}
}

func TestDeletingComponentResourceProducesResourceOutputsEvent(t *testing.T) {
	t.Parallel()

	cancelCtx, _ := cancel.NewContext(context.Background())

	acts := newUpdateActions(&Context{
		Cancel: cancelCtx,
	}, nil, &deploymentOptions{})
	eventsChan := make(chan Event, 10)
	acts.Opts.Events.ch = eventsChan

	step := deploy.NewDeleteStep(&deploy.Deployment{}, &sync.Map{}, &resource.State{
		URN:      resource.URN("urn:pulumi:stack::project::my:example:Foo::foo"),
		ID:       "foo",
		Custom:   false,
		Provider: "unimportant",
	})
	acts.Seen[resource.URN("urn:pulumi:stack::project::my:example:Foo::foo")] = step

	err := acts.OnResourceStepPost(
		&mockSnapshotMutation{}, step, resource.StatusOK,
		nil, /* err */
	)
	require.NoError(t, err)

	//nolint:exhaustive // the default case is for test failures
	switch e := <-eventsChan; e.Type {
	case ResourceOutputsEvent:
		e, ok := e.Payload().(ResourceOutputsEventPayload)
		assert.True(t, ok)
		assert.True(t, e.Metadata.URN == "urn:pulumi:stack::project::my:example:Foo::foo")
	default:
		assert.Fail(t, "unexpected event type")
	}
}

type mockSnapshotMutation struct{}

func (msm *mockSnapshotMutation) End(step deploy.Step, successful bool) error { return nil }
