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

package fsa_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/util/fsa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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

	e1 = machine.NewEdge(func(ctx context.Context, m fsa.FSA[string], v string) (fsa.ConditionResult, error) {
		assert.Equal(t, "v1", v)
		actions = append(actions, "n0->n1")
		return fsa.ConditionPass, nil
	}, n0, n1)

	machine.NewCursor("v1", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

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

	n1 := machine.NewNode(func(ctx context.Context, m fsa.FSA[string], edge fsa.Edge, v string) error {
		panic("should not be called")
	})

	machine.NewEdge(func(ctx context.Context, m fsa.FSA[string], v string) (fsa.ConditionResult, error) {
		assert.Equal(t, "v1", v)
		return fsa.ConditionFail, nil
	}, n0, n1)

	machine.NewCursor("v1", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	assert.Equal(t, map[string]fsa.Node{
		"v1": n0,
	}, maps.Collect(machine.Parked))
	assert.Equal(t, map[string]fsa.Node{
		"v1": n0,
	}, maps.Collect(machine.Cursors))
}

func nopNode[Cursor any](context.Context, fsa.FSA[Cursor], fsa.Edge, Cursor) error { return nil }

// A condition that passes exactly once, then fails.
func passOnce[Cursor any]() fsa.ConditionFunc[Cursor] {
	used := false
	return func(context.Context, fsa.FSA[Cursor], Cursor) (fsa.ConditionResult, error) {
		if used {
			return fsa.ConditionFail, nil
		}
		used = true
		return fsa.ConditionPass, nil
	}
}

func asyncRunner(t *testing.T) (fsa.Runner, context.Context) {
	g, c := errgroup.WithContext(t.Context())

	return func(ctx context.Context, f func(context.Context)) error {
		g.Go(func() error {
			f(ctx)
			return nil
		})
		return nil
	}, c
}

// A cursor arriving at an occupied node must let the occupant escape first.
func TestOccupantEscapes(t *testing.T) {
	t.Parallel()

	machine := fsa.New[string]()
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(nopNode[string])
	n2 := machine.NewNode(nopNode[string])
	machine.NewEdge(passOnce[string](), n0, n1)
	machine.NewEdge(passOnce[string](), n1, n2)

	machine.NewCursor("x", n0)
	machine.NewCursor("y", n1)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	assert.Equal(t, map[string]fsa.Node{
		"x": n1,
		"y": n2,
	}, maps.Collect(machine.Cursors))
}

// A cursor arriving at a node whose occupant cannot move overwrites the occupant, which is reported to
// the OnReplace hook.
func TestStuckOccupantOverwritten(t *testing.T) {
	t.Parallel()

	type replaced struct {
		old, by string
		at      fsa.Node
	}
	var replacements []replaced
	machine := fsa.New(fsa.OnReplace(func(old, by string, at fsa.Node) {
		replacements = append(replacements, replaced{old, by, at})
	}))
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(nopNode[string])
	n2 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	machine.NewEdge(passOnce[string](), n0, n1)
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionFail, nil
	}, n1, n2)

	machine.NewCursor("x", n0)
	machine.NewCursor("y", n1)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	assert.Equal(t, map[string]fsa.Node{
		"x": n1,
	}, maps.Collect(machine.Cursors))
	assert.Equal(t, map[string]fsa.Node{
		"x": n1,
	}, maps.Collect(machine.Parked))
	assert.Equal(t, []replaced{{"y", "x", n1}}, replacements)
}

// Two cursors concurrently moving to the same node is a race and reported as an error.
func TestRacingArrivalsError(t *testing.T) {
	t.Parallel()

	machine := fsa.New[string]()
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(nopNode[string])
	n2 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	pass := func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionPass, nil
	}
	machine.NewEdge(pass, n0, n2)
	machine.NewEdge(pass, n1, n2)

	machine.NewCursor("x", n0)
	machine.NewCursor("y", n1)

	err := machine.Progress(t.Context(), fsa.SyncRunner)
	assert.ErrorContains(t, err, "both moving")

	assert.Equal(t, map[string]fsa.Node{
		"x": n0,
		"y": n1,
	}, maps.Collect(machine.Cursors))
}

// Cursors deferred on each other's nodes (a cycle) all commit: each occupant can move, so each is
// guaranteed the chance to.
func TestCycleRotates(t *testing.T) {
	t.Parallel()

	var entered []string

	machine := fsa.New[string]()
	var n0, n1 fsa.Node
	n0 = machine.NewNode(func(_ context.Context, _ fsa.FSA[string], _ fsa.Edge, v string) error {
		entered = append(entered, "n0:"+v)
		return nil
	})
	n1 = machine.NewNode(func(_ context.Context, _ fsa.FSA[string], _ fsa.Edge, v string) error {
		entered = append(entered, "n1:"+v)
		return nil
	})
	machine.NewEdge(passOnce[string](), n0, n1)
	machine.NewEdge(passOnce[string](), n1, n0)

	machine.NewCursor("x", n0)
	machine.NewCursor("y", n1)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	assert.Equal(t, []string{"n1:x", "n0:y"}, entered)
	assert.Equal(t, map[string]fsa.Node{
		"x": n1,
		"y": n0,
	}, maps.Collect(machine.Cursors))
}

// A real failure cancels outstanding work; the context.Canceled collateral that produces is swallowed so
// Progress reports only the cause.
func TestFailureSwallowsCancellationCollateral(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	machine := fsa.New[string]()
	pass := func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionPass, nil
	}

	slowSrc := machine.NewNode(nopNode[string])
	slowDst := machine.NewNode(func(ctx context.Context, _ fsa.FSA[string], _ fsa.Edge, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	})
	machine.NewEdge(pass, slowSrc, slowDst)
	machine.NewCursor("slow", slowSrc)

	boomSrc := machine.NewNode(nopNode[string])
	boomDst := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		return errBoom
	})
	machine.NewEdge(pass, boomSrc, boomDst)
	machine.NewCursor("boom", boomSrc)

	goRunner := func(ctx context.Context, f func(context.Context)) error {
		go f(ctx)
		return nil
	}

	err := machine.Progress(t.Context(), goRunner)
	assert.ErrorIs(t, err, errBoom)
	assert.NotErrorIs(t, err, context.Canceled)
}

// External cancellation surfaces the cancellation's cause, not the bare context.Canceled it produces.
func TestExternalCancelReportsCause(t *testing.T) {
	t.Parallel()

	errShutdown := errors.New("shutting down")
	parent, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	machine := fsa.New[string]()
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(func(ctx context.Context, _ fsa.FSA[string], _ fsa.Edge, _ string) error {
		cancel(errShutdown)
		<-ctx.Done()
		return ctx.Err()
	})
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionPass, nil
	}, n0, n1)
	machine.NewCursor("x", n0)

	err := machine.Progress(parent, fsa.SyncRunner)
	assert.ErrorIs(t, err, errShutdown)
	assert.NotErrorIs(t, err, context.Canceled)
}

// Nodes, edges, and cursors added while entering a node are live within the same Progress call: a new
// cursor is progressed, and a new edge is retried by cursors stuck at its source — whether they are
// parked (all conditions failed) or resting on a node that had no outgoing edges.
func TestMutationDuringProgress(t *testing.T) {
	t.Parallel()

	var entered []string
	record := func(name string) fsa.NodeFunc[string] {
		return func(_ context.Context, _ fsa.FSA[string], _ fsa.Edge, v string) error {
			entered = append(entered, name+":"+v)
			return nil
		}
	}

	machine := fsa.New[string]()

	// "parked" sits on a node whose only edge always fails.
	parkedAt := machine.NewNode(nopNode[string])
	deadEnd := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionFail, nil
	}, parkedAt, deadEnd)
	machine.NewCursor("parked", parkedAt)

	// "resting" sits on a node with no outgoing edges at all.
	restingAt := machine.NewNode(nopNode[string])
	machine.NewCursor("resting", restingAt)

	// "late" is created mid-progress on lateStart, whose edge out already exists.
	lateStart := machine.NewNode(nopNode[string])
	lateEnd := machine.NewNode(record("lateEnd"))
	machine.NewEdge(passOnce[string](), lateStart, lateEnd)

	// Entering mutator adds two nodes, an edge to them from each stuck cursor's location, and a new
	// cursor.
	var fromParked, fromResting fsa.Node
	n0 := machine.NewNode(nopNode[string])
	mutator := machine.NewNode(func(_ context.Context, m fsa.FSA[string], _ fsa.Edge, v string) error {
		entered = append(entered, "mutator:"+v)
		fromParked = m.NewNode(record("fromParked"))
		fromResting = m.NewNode(record("fromResting"))
		m.NewEdge(passOnce[string](), parkedAt, fromParked)
		m.NewEdge(passOnce[string](), restingAt, fromResting)
		m.NewCursor("late", lateStart)
		return nil
	})
	machine.NewEdge(passOnce[string](), n0, mutator)
	machine.NewCursor("x", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	slices.Sort(entered)
	assert.Equal(t, []string{
		"fromParked:parked",
		"fromResting:resting",
		"lateEnd:late",
		"mutator:x",
	}, entered)
	assert.Equal(t, map[string]fsa.Node{
		"x":       mutator,
		"parked":  fromParked,
		"resting": fromResting,
		"late":    lateEnd,
	}, maps.Collect(machine.Cursors))
	assert.Equal(t, map[string]fsa.Node{}, maps.Collect(machine.Parked))
}

// A condition that adds edges and then fails must not strand cursors: the new edges are tried even
// though no move (and so no generation bump) ever follows — both by the evaluating cursor itself and by
// an already-parked cursor at a new edge's source. Edges that already failed are not re-evaluated.
func TestEdgeAddedByFailingCondition(t *testing.T) {
	t.Parallel()

	var entered []string
	record := func(name string) fsa.NodeFunc[string] {
		return func(_ context.Context, _ fsa.FSA[string], _ fsa.Edge, v string) error {
			entered = append(entered, name+":"+v)
			return nil
		}
	}

	machine := fsa.New[string]()

	// "b" parks: its only edge always fails.
	nB := machine.NewNode(nopNode[string])
	bDead := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionFail, nil
	}, nB, bDead)
	machine.NewCursor("b", nB)

	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	n2 := machine.NewNode(record("n2"))
	n3 := machine.NewNode(record("n3"))

	// x's only condition adds an edge from x's own node and one from b's node, then fails.
	condCalls := 0
	machine.NewEdge(func(_ context.Context, m fsa.FSA[string], _ string) (fsa.ConditionResult, error) {
		condCalls++
		if condCalls == 1 {
			m.NewEdge(passOnce[string](), n0, n2)
			m.NewEdge(passOnce[string](), nB, n3)
		}
		return fsa.ConditionFail, nil
	}, n0, n1)
	machine.NewCursor("x", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))

	slices.Sort(entered)
	assert.Equal(t, []string{"n2:x", "n3:b"}, entered)
	assert.Equal(t, map[string]fsa.Node{
		"b": n3,
		"x": n2,
	}, maps.Collect(machine.Cursors))
	assert.Equal(t, map[string]fsa.Node{}, maps.Collect(machine.Parked))
	assert.Equal(t, 1, condCalls)
}

// Every Progress call starts a fresh visit: a condition that failed in one call is re-asked by the
// next, even though the cursor never moved.
func TestProgressRestartsVisits(t *testing.T) {
	t.Parallel()

	machine := fsa.New[string]()
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(func(context.Context, fsa.FSA[string], fsa.Edge, string) error {
		panic("should not be called")
	})
	condCalls := 0
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		condCalls++
		return fsa.ConditionFail, nil
	}, n0, n1)
	machine.NewCursor("x", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
	assert.Equal(t, 1, condCalls)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
	assert.Equal(t, 2, condCalls)

	assert.Equal(t, map[string]fsa.Node{
		"x": n0,
	}, maps.Collect(machine.Parked))
}

// Entry functions run in parallel under an async runner: the two entry functions rendezvous, each
// blocking until the other has started, so Progress only completes if the runner overlaps them.
func TestEntryFunctionsRunInParallel(t *testing.T) {
	t.Parallel()

	machine := fsa.New[string]()
	pass := func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionPass, nil
	}

	xArrived := make(chan struct{})
	yArrived := make(chan struct{})
	rendezvous := func(arrive chan<- struct{}, await <-chan struct{}) fsa.NodeFunc[string] {
		return func(ctx context.Context, _ fsa.FSA[string], _ fsa.Edge, _ string) error {
			close(arrive)
			select {
			case <-await:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	xSrc := machine.NewNode(nopNode[string])
	xDst := machine.NewNode(rendezvous(xArrived, yArrived))
	machine.NewEdge(pass, xSrc, xDst)
	machine.NewCursor("x", xSrc)

	ySrc := machine.NewNode(nopNode[string])
	yDst := machine.NewNode(rendezvous(yArrived, xArrived))
	machine.NewEdge(pass, ySrc, yDst)
	machine.NewCursor("y", ySrc)

	runner, ctx := asyncRunner(t)
	require.NoError(t, machine.Progress(ctx, runner))

	assert.Equal(t, map[string]fsa.Node{
		"x": xDst,
		"y": yDst,
	}, maps.Collect(machine.Cursors))
}

// Independent cursors progress correctly when the runner spreads work across goroutines.
func TestConcurrentRunner(t *testing.T) {
	t.Parallel()

	const count = 32

	machine := fsa.New[int]()
	var mu sync.Mutex
	entered := map[int]int{}

	want := map[int]fsa.Node{}
	for i := range count {
		src := machine.NewNode(func(context.Context, fsa.FSA[int], fsa.Edge, int) error {
			panic("should not be called")
		})
		dst := machine.NewNode(func(_ context.Context, _ fsa.FSA[int], _ fsa.Edge, v int) error {
			mu.Lock()
			defer mu.Unlock()
			entered[v]++
			return nil
		})
		machine.NewEdge(func(context.Context, fsa.FSA[int], int) (fsa.ConditionResult, error) {
			return fsa.ConditionPass, nil
		}, src, dst)
		machine.NewCursor(i, src)
		want[i] = dst
	}

	goRunner := func(ctx context.Context, f func(context.Context)) error {
		go f(ctx)
		return nil
	}

	require.NoError(t, machine.Progress(t.Context(), goRunner))

	assert.Equal(t, want, maps.Collect(machine.Cursors))
	wantEntered := map[int]int{}
	for i := range count {
		wantEntered[i] = 1
	}
	assert.Equal(t, wantEntered, entered)
}

// A run that fails still commits every cursor whose entry function returned nil. An occupant of the
// entered node that the abort left unsettled is not overwritten yet: it stays beside the arrival and, on
// the next Progress, either moves away or is overwritten once it settles.
func TestSucceededEntryCommitsDuringFailure(t *testing.T) {
	t.Parallel()

	for _, occupantEscapes := range []bool{true, false} {
		t.Run(fmt.Sprintf("occupantEscapes=%t", occupantEscapes), func(t *testing.T) {
			t.Parallel()

			type replaced struct {
				old, by string
				at      fsa.Node
			}
			var replacements []replaced
			machine := fsa.New(fsa.OnReplace(func(old, by string, at fsa.Node) {
				replacements = append(replacements, replaced{old, by, at})
			}))
			var nY fsa.Node
			n0 := machine.NewNode(nopNode[string])
			n1 := machine.NewNode(nopNode[string])
			nZ := machine.NewNode(nopNode[string])
			// Entering nX fails the run — after placing an occupant on nY that the abort will leave
			// unsettled, since no condition is started once the run is failing.
			nX := machine.NewNode(func(_ context.Context, m fsa.FSA[string], _ fsa.Edge, _ string) error {
				m.NewCursor("occupant", nY)
				return errors.New("entry exploded")
			})
			nY = machine.NewNode(nopNode[string])
			machine.NewEdge(passOnce[string](), n0, nX)
			machine.NewEdge(passOnce[string](), n1, nY)
			machine.NewEdge(func(_ context.Context, _ fsa.FSA[string], v string) (fsa.ConditionResult, error) {
				if v == "occupant" && occupantEscapes {
					return fsa.ConditionPass, nil
				}
				return fsa.ConditionFail, nil
			}, nY, nZ)
			// The mover's entry is dispatched first, so it completes before the failure aborts the run.
			machine.NewCursor("mover", n1)
			machine.NewCursor("failing", n0)

			require.EqualError(t, machine.Progress(t.Context(), fsa.SyncRunner), "entry exploded")
			assert.Equal(t, map[string]fsa.Node{
				"failing":  n0,
				"mover":    nY, // Committed although the run failed
				"occupant": nY, // Not overwritten yet: it has not had its chance to move
			}, maps.Collect(machine.Cursors))
			assert.Empty(t, replacements)

			require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
			if occupantEscapes {
				assert.Equal(t, map[string]fsa.Node{
					"failing":  n0,
					"mover":    nY,
					"occupant": nZ,
				}, maps.Collect(machine.Cursors))
				assert.Empty(t, replacements)
			} else {
				assert.Equal(t, map[string]fsa.Node{
					"failing": n0,
					"mover":   nY,
				}, maps.Collect(machine.Cursors))
				assert.Equal(t, []replaced{{"occupant", "mover", nY}}, replacements)
			}
		})
	}
}

// Placing a cursor on an occupied node is an arrival: the occupant is overwritten once it settles
// without having moved away, which the next Progress decides.
func TestPlacementIsAnArrival(t *testing.T) {
	t.Parallel()

	for _, occupantEscapes := range []bool{true, false} {
		t.Run(fmt.Sprintf("occupantEscapes=%t", occupantEscapes), func(t *testing.T) {
			t.Parallel()

			var replacements []string
			machine := fsa.New(fsa.OnReplace(func(old, by string, _ fsa.Node) {
				replacements = append(replacements, old+" by "+by)
			}))
			n0 := machine.NewNode(nopNode[string])
			n1 := machine.NewNode(nopNode[string])
			machine.NewEdge(func(_ context.Context, _ fsa.FSA[string], v string) (fsa.ConditionResult, error) {
				if v == "occupant" && occupantEscapes {
					return fsa.ConditionPass, nil
				}
				return fsa.ConditionFail, nil
			}, n0, n1)
			machine.NewCursor("occupant", n0)
			machine.NewCursor("arrival", n0)
			var order []string
			for c := range machine.Cursors {
				order = append(order, c)
			}
			assert.Equal(t, []string{"occupant", "arrival"}, order, "Cursors iterates in arrival order")

			require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
			if occupantEscapes {
				assert.Equal(t, map[string]fsa.Node{"occupant": n1, "arrival": n0}, maps.Collect(machine.Cursors))
				assert.Empty(t, replacements)
			} else {
				assert.Equal(t, map[string]fsa.Node{"arrival": n0}, maps.Collect(machine.Cursors))
				assert.Equal(t, []string{"occupant by arrival"}, replacements)
			}
		})
	}
}

// Placing a cursor during Progress on a node whose occupant is already settled overwrites it at once.
func TestPlacementOverwritesSettledOccupant(t *testing.T) {
	t.Parallel()

	var replacements []string
	machine := fsa.New(fsa.OnReplace(func(old, by string, _ fsa.Node) {
		replacements = append(replacements, old+" by "+by)
	}))
	n0 := machine.NewNode(nopNode[string])
	nX := machine.NewNode(nopNode[string])
	nY := machine.NewNode(func(_ context.Context, m fsa.FSA[string], _ fsa.Edge, _ string) error {
		m.NewCursor("arrival", n0) // The occupant of n0 has already parked by now
		return nil
	})
	machine.NewEdge(func(context.Context, fsa.FSA[string], string) (fsa.ConditionResult, error) {
		return fsa.ConditionFail, nil
	}, n0, nX)
	machine.NewEdge(passOnce[string](), nX, nY)
	machine.NewCursor("occupant", n0)
	machine.NewCursor("placer", nX)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
	assert.Equal(t, map[string]fsa.Node{"arrival": n0, "placer": nY}, maps.Collect(machine.Cursors))
	assert.Equal(t, []string{"occupant by arrival"}, replacements)
}

// Removed cursors cease without an overwrite; work in flight for them is discarded.
func TestRemove(t *testing.T) {
	t.Parallel()

	var replacements []string
	machine := fsa.New(fsa.OnReplace(func(old, by string, _ fsa.Node) {
		replacements = append(replacements, old+" by "+by)
	}))
	var entered []string
	n0 := machine.NewNode(nopNode[string])
	n1 := machine.NewNode(func(_ context.Context, _ fsa.FSA[string], _ fsa.Edge, v string) error {
		entered = append(entered, v)
		return nil
	})
	var removeMe fsa.CursorRef
	machine.NewEdge(func(_ context.Context, m fsa.FSA[string], v string) (fsa.ConditionResult, error) {
		if v == "remover" {
			// The other cursor's condition for this edge is in the same batch: its pass is discarded.
			require.True(t, m.Remove(removeMe))
			require.False(t, m.Remove(removeMe), "already gone")
		}
		return fsa.ConditionPass, nil
	}, n0, n1)
	machine.NewCursor("remover", n0)
	removeMe = machine.NewCursor("removed", n0)

	require.NoError(t, machine.Progress(t.Context(), fsa.SyncRunner))
	assert.Equal(t, map[string]fsa.Node{"remover": n1}, maps.Collect(machine.Cursors))
	assert.Equal(t, []string{"remover"}, entered)
	assert.Empty(t, replacements)
}
