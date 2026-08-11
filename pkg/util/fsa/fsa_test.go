package fsa_test

import (
	"context"
	"maps"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnblockedProgression(t *testing.T) {
	t.Parallel()

	var actions []string

	machine := fsa.New[string]()
	var e1 fsa.Edge
	n0 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})

	var n1 fsa.Node
	n1 = machine.NewNode(func(ctx context.Context, m fsa.FSA[string], edge fsa.Edge, v string) error {
		from, to := m.GetEdge(edge)
		assert.Equal(t, from, n0)
		assert.Equal(t, to, n1)
		assert.Equal(t, edge, e1)
		assert.Equal(t, "v1", v)
		actions = append(actions, "n1")
		return nil
	})

	e1 = machine.NewEdge(func(ctx context.Context, m fsa.FSA[string], from, to fsa.Node) (fsa.ConditionResult, error) {
		assert.Equal(t, n0, from)
		assert.Equal(t, n1, to)
		actions = append(actions, "n0->n1")
		return fsa.ConditionPass, nil
	}, n0, n1)

	machine.NewCursor("v1", n0)

	require.NoError(t, machine.Progress(t.Context()))

	assert.Equal(t, []string{
		"n0->n1",
		"n1",
	}, actions)

	assert.Equal(t, map[string]fsa.Node{}, maps.Collect(machine.Parked))
	assert.Equal(t, map[string]fsa.Node{
		"v1": n1,
	}, maps.Collect(machine.Cursors))
}

func TestBlockedProgression(t *testing.T) {
	t.Parallel()

	machine := fsa.New[string]()
	n0 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})

	var n1 fsa.Node
	n1 = machine.NewNode(func(ctx context.Context, m fsa.FSA[string], edge fsa.Edge, v string) error {
		panic("should not be called")
	})

	machine.NewEdge(func(ctx context.Context, m fsa.FSA[string], from, to fsa.Node) (fsa.ConditionResult, error) {
		assert.Equal(t, n0, from)
		assert.Equal(t, n1, to)
		return fsa.ConditionFail, nil
	}, n0, n1)

	machine.NewCursor("v1", n0)

	require.NoError(t, machine.Progress(t.Context()))

	assert.Equal(t, map[string]fsa.Node{
		"v1": n0,
	}, maps.Collect(machine.Parked))
	assert.Equal(t, map[string]fsa.Node{
		"v1": n0,
	}, maps.Collect(machine.Cursors))
}
