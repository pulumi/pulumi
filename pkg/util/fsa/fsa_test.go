package fsa_test

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/stretchr/testify/assert"
)

func TestUnblockedProgression(t *testing.T) {
	t.Parallel()

	var actions []string

	machine := fsa.New[string]()
	var e1 fsa.Edge
	n1 := machine.NewNode(func(ctx context.Context, m fsa.FSA[string], edge fsa.Edge, v string) error {
		assert.Equal(t, "v1", v)
		actions = append(actions, "node1")
		return nil
	})

	var n2 fsa.Node
	n2 = machine.NewNode(func(ctx context.Context, m fsa.FSA[string], edge fsa.Edge, v string) error {
		from, to := fsa.GetEdge(edge)
		assert.Equal(t, from, n1)
		assert.Equal(t, to, n2)
		assert.Equal(t, edge, e1)
		assert.Equal(t, "v1", v)
		actions = append(actions, "node2")
		return nil
	})

	e1 = fsa.NewEdge(func(ctx context.Context, fsa fsa.FSA[Cursor], from, to fsa.Node) (fsa.ConditionResult, error) {
		assert.Equal(t, n1, from)
		assert.Equal(t, n2, to)
		actions = append(actions, "n1->n2")
		return fsa.ConditionPass, nil
	}, n1, n2)

	fsa.NewCursor("v1", n1)
}
