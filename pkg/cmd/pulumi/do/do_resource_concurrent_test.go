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

package do

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerAlreadyGone is what a provider reports when asked to delete something that is no longer
// there. Note what it lacks: any marker a caller could classify as "not found".
const providerAlreadyGone = "api error InvalidPermission.NotFound: the specified rule does not exist"

// racingCloud is a single backing store shared by concurrent delete invocations — the stand-in for
// the cloud itself, which is the only thing two `pulumi do` processes actually share.
type racingCloud struct {
	mu   sync.Mutex
	live map[resource.ID]resource.PropertyMap

	reads   atomic.Int32
	deletes atomic.Int32

	// preflight is closed once both invocations' first Read has landed. Holding both there until
	// they have each seen the resource forces the interleaving we care about — both pass their
	// pre-flight check, then both attempt the delete — instead of leaving it to chance.
	preflight     chan struct{}
	preflightSize int32
}

func newRacingCloud(ids ...resource.ID) *racingCloud {
	c := &racingCloud{
		live:          map[resource.ID]resource.PropertyMap{},
		preflight:     make(chan struct{}),
		preflightSize: 2,
	}
	for _, id := range ids {
		c.live[id] = resource.PropertyMap{"name": resource.NewProperty(string(id))}
	}
	return c
}

func (c *racingCloud) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	// Only the pre-flight reads synchronise. Any later read — the one issued after a failed delete
	// — must not wait, or it would deadlock behind a barrier that is already satisfied.
	if n := c.reads.Add(1); n <= c.preflightSize {
		if n == c.preflightSize {
			close(c.preflight)
		}
		select {
		case <-c.preflight:
		case <-time.After(30 * time.Second):
			// Fail via assertions rather than hanging the suite forever.
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.live[req.ID]
	if !ok {
		// Gone — and in the #23916 shape: the requested ID echoed back over emptied state.
		return plugin.ReadResponse{ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
		}}, nil
	}
	return plugin.ReadResponse{ReadResult: plugin.ReadResult{
		ID: req.ID, Inputs: state.Copy(), Outputs: state.Copy(),
	}}, nil
}

func (c *racingCloud) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	c.deletes.Add(1)

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.live[req.ID]; !ok {
		return plugin.DeleteResponse{}, errors.New(providerAlreadyGone)
	}
	delete(c.live, req.ID)
	return plugin.DeleteResponse{}, nil
}

func (c *racingCloud) stillLive(id resource.ID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.live[id]
	return ok
}

// TestDoCmdResourceConcurrentDelete runs two delete invocations against one backing store at the
// same time, with both held at their pre-flight Read until each has seen the resource. One wins the
// delete; the other must come back classifiable as not-found rather than surfacing the provider's
// own already-gone error.
func TestDoCmdResourceConcurrentDelete(t *testing.T) {
	t.Parallel()

	const invocations = 2
	cloud := newRacingCloud("res-1")
	provider := &testProvider{
		spec: doResourceSpec(false),
		MockProvider: plugin.MockProvider{
			ReadF:   cloud.Read,
			DeleteF: cloud.Delete,
		},
	}

	errs := make([]error, invocations)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each invocation gets its own command tree, as it would its own process; the only
			// thing they share is the cloud behind the provider.
			cmd, _, _ := newDoResourceCommand(t, provider)
			cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "res-1", "--yes"})
			errs[i] = cmd.Execute()
		}()
	}
	wg.Wait()

	var succeeded int
	var failures []error
	for _, err := range errs {
		if err == nil {
			succeeded++
			continue
		}
		failures = append(failures, err)
	}

	assert.Equal(t, 1, succeeded, "exactly one invocation should win the delete")
	require.Len(t, failures, 1, "exactly one invocation should lose the race")

	// The loser must be classifiable. Before the re-read it surfaced the provider's own error,
	// which carries nothing a caller could branch on.
	assert.ErrorContains(t, failures[0], `resource "res-1" was not found`)
	assert.NotContains(t, failures[0].Error(), providerAlreadyGone)

	// Both invocations really did attempt the delete — i.e. they actually raced rather than
	// serialising by luck, which would make the assertions above vacuous.
	assert.Equal(t, int32(invocations), cloud.deletes.Load(), "both invocations should reach Delete")

	// The hard bound: two pre-flight reads plus exactly one re-read by the loser. If the re-read
	// ever grew into a retry loop, this count would climb.
	assert.Equal(t, int32(invocations+1), cloud.reads.Load(),
		"expected one re-read by the losing invocation and no more")

	assert.False(t, cloud.stillLive("res-1"), "the resource should be gone from the backing store")

	// The signal a caller actually branches on. Matching the message text is not a stable contract
	// across releases; the exit code is.
	assert.Equal(t, cmdCmd.ExitResourceNotFound, cmdCmd.ExitCodeFor(failures[0]),
		"a resource deleted out from under us should exit as not-found, not a generic error")
}

// TestDoCmdResourcePatchDeletedMidFlight covers patch losing the same race delete can lose: the
// pre-flight Read finds the resource, it is deleted before the Update lands, and the provider
// answers with an error carrying nothing to classify. Patch has nothing left to do once the
// resource is gone, so it must reach the caller as not-found, exactly as delete does.
func TestDoCmdResourcePatchDeletedMidFlight(t *testing.T) {
	t.Parallel()

	const updateFailure = "api error ResourceNotFoundException: no such resource"

	tests := map[string]struct {
		// gone says whether the resource has vanished by the time the post-failure re-read runs.
		gone           bool
		wantNotFound   bool
		wantExitCode   int
		wantErrContain string
	}{
		"deleted between the read and the update": {
			gone:           true,
			wantNotFound:   true,
			wantExitCode:   cmdCmd.ExitResourceNotFound,
			wantErrContain: `resource "res-1" was not found`,
		},
		"update failed for its own reasons": {
			gone:           false,
			wantNotFound:   false,
			wantExitCode:   cmdCmd.ExitCodeError,
			wantErrContain: updateFailure,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var reads atomic.Int32
			present := plugin.ReadResponse{ReadResult: plugin.ReadResult{
				ID:      "res-1",
				Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
				Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
			}}
			cmd, _, _ := newDoResourceCommand(t, &testProvider{
				spec: doResourceSpec(false),
				MockProvider: plugin.MockProvider{
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						if reads.Add(1) == 1 || !tt.gone {
							return present, nil
						}
						// The post-failure re-read, after a concurrent delete: emptied state.
						return plugin.ReadResponse{ReadResult: plugin.ReadResult{
							ID:      req.ID,
							Inputs:  resource.PropertyMap{},
							Outputs: resource.PropertyMap{},
						}}, nil
					},
					CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
						return plugin.CheckResponse{Properties: req.News}, nil
					},
					DiffF: func(_ context.Context, _ plugin.DiffRequest) (plugin.DiffResponse, error) {
						return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
					},
					UpdateF: func(_ context.Context, _ plugin.UpdateRequest) (plugin.UpdateResponse, error) {
						return plugin.UpdateResponse{}, errors.New(updateFailure)
					},
				},
			})
			inputFile := writeHCLFile(t, "patch.pcl", `enabled = true`)
			cmd.SetArgs([]string{
				"--stateless", "azure:index:myResource", "patch", "res-1", "--yes",
				"--input", "pcl", "--input-file", inputFile,
			})
			err := cmd.Execute()

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErrContain)
			assert.Equal(t, tt.wantExitCode, cmdCmd.ExitCodeFor(err))
			// Same structural bound as delete: pre-flight plus exactly one re-read, never a retry.
			assert.Equal(t, int32(2), reads.Load(), "exactly one re-read, and no Update retry")
		})
	}
}

// TestDoCmdResourceUpsertKeepsFailureWhenDeletedMidFlight pins upsert's deliberate opt-out. Upsert
// creates what it does not find, so "not found" is not one of its outcomes — answering with it
// would tell a caller to stop reconciling when the correct response is to run again and create.
func TestDoCmdResourceUpsertKeepsFailureWhenDeletedMidFlight(t *testing.T) {
	t.Parallel()

	const updateFailure = "api error ResourceNotFoundException: no such resource"

	var reads atomic.Int32
	cmd, _, _ := newDoResourceCommand(t, &testProvider{
		spec: doResourceSpec(false),
		MockProvider: plugin.MockProvider{
			ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
				if reads.Add(1) == 1 {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
					}}, nil
				}
				return plugin.ReadResponse{ReadResult: plugin.ReadResult{
					ID:      req.ID,
					Inputs:  resource.PropertyMap{},
					Outputs: resource.PropertyMap{},
				}}, nil
			},
			CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
				return plugin.CheckResponse{Properties: req.News}, nil
			},
			DiffF: func(_ context.Context, _ plugin.DiffRequest) (plugin.DiffResponse, error) {
				return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
			},
			UpdateF: func(_ context.Context, _ plugin.UpdateRequest) (plugin.UpdateResponse, error) {
				return plugin.UpdateResponse{}, errors.New(updateFailure)
			},
		},
	})
	inputFile := writeHCLFile(t, "upsert.pcl", `name = "new"`)
	cmd.SetArgs([]string{
		"--stateless", "azure:index:myResource", "upsert", "res-1", "--yes",
		"--input", "pcl", "--input-file", inputFile,
	})
	err := cmd.Execute()

	require.Error(t, err)
	assert.ErrorContains(t, err, updateFailure)
	assert.NotContains(t, strings.ToLower(err.Error()), "was not found")
	assert.Equal(t, cmdCmd.ExitCodeError, cmdCmd.ExitCodeFor(err),
		"upsert must stay retryable so the next run creates the resource")
	// No re-read at all: upsert opts out, so the failure is reported as-is.
	assert.Equal(t, int32(1), reads.Load())
}

// TestDoCmdResourceNotFoundExitCode pins the exit code contract for every operation that reports a
// missing resource, including through the wrapping the CLI applies on the way out. A caller keying
// off the code must not have to care which path produced it.
func TestDoCmdResourceNotFoundExitCode(t *testing.T) {
	t.Parallel()

	// The emptied read that started all this: an echoed ID over no state.
	emptiedRead := func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
		return plugin.ReadResponse{ReadResult: plugin.ReadResult{
			ID:      req.ID,
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
		}}, nil
	}

	operations := map[string][]string{
		"delete": {"--stateless", "azure:index:myResource", "delete", "res-gone", "--yes"},
		"read":   {"azure:index:myResource", "read", "res-gone"},
	}

	for name, args := range operations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cmd, _, _ := newDoResourceCommand(t, &testProvider{
				spec:         doResourceSpec(false),
				MockProvider: plugin.MockProvider{ReadF: emptiedRead},
			})
			cmd.SetArgs(args)
			err := cmd.Execute()

			require.Error(t, err)
			// Message unchanged, so callers matching on text keep working.
			assert.ErrorContains(t, err, `resource "res-gone" was not found`)
			assert.Equal(t, cmdCmd.ExitResourceNotFound, cmdCmd.ExitCodeFor(err))

			// The code has to survive wrapping: errors pick up context and a BailError envelope
			// between RunE and the exit, and a bare type assertion would lose it there.
			assert.Equal(t, cmdCmd.ExitResourceNotFound,
				cmdCmd.ExitCodeFor(fmt.Errorf("delete failed: %w", err)),
				"the exit code must survive error wrapping")
		})
	}

	t.Run("a genuine failure stays a generic error", func(t *testing.T) {
		t.Parallel()
		cmd, _, _ := newDoResourceCommand(t, &testProvider{
			spec: doResourceSpec(false),
			MockProvider: plugin.MockProvider{
				ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID:      req.ID,
						Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
						Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
					}}, nil
				},
				DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
					return plugin.DeleteResponse{}, errors.New("api error DependencyViolation: resource is in use")
				},
			},
		})
		cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "res-1", "--yes"})
		err := cmd.Execute()

		require.Error(t, err)
		assert.Equal(t, cmdCmd.ExitCodeError, cmdCmd.ExitCodeFor(err),
			"a real failure must stay retryable, not be reported as already-gone")
	})
}

// TestDoCmdResourceFailedDeleteKeepsOriginalError covers the other direction: the re-read must only
// ever downgrade a delete failure to not-found on positive evidence of absence. Anything ambiguous
// — state still present, or a re-read that itself fails — has to leave the original error intact,
// or a genuine failure would be reported to the caller as a successful cleanup.
func TestDoCmdResourceFailedDeleteKeepsOriginalError(t *testing.T) {
	t.Parallel()

	const deleteFailure = "api error DependencyViolation: resource is in use"

	tests := map[string]struct {
		// reread is the response to the Read issued after the failed Delete.
		reread func() (plugin.ReadResponse, error)
	}{
		"resource is still there": {
			reread: func() (plugin.ReadResponse, error) {
				return plugin.ReadResponse{ReadResult: plugin.ReadResult{
					ID:      "res-1",
					Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
					Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
				}}, nil
			},
		},
		// An eventually-consistent backend part-way through settling. Ambiguous, so the original
		// error stands and the caller's own retry picks up the clean answer next time round.
		"resource reads back as partial state": {
			reread: func() (plugin.ReadResponse, error) {
				return plugin.ReadResponse{ReadResult: plugin.ReadResult{
					ID:      "res-1",
					Inputs:  resource.PropertyMap{},
					Outputs: resource.PropertyMap{"name": resource.NewProperty("half")},
				}}, nil
			},
		},
		"re-read itself fails": {
			reread: func() (plugin.ReadResponse, error) {
				return plugin.ReadResponse{}, errors.New("api error Throttling: rate exceeded")
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var reads atomic.Int32
			cmd, _, _ := newDoResourceCommand(t, &testProvider{
				spec: doResourceSpec(false),
				MockProvider: plugin.MockProvider{
					ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
						if reads.Add(1) == 1 {
							// Pre-flight: the resource is there, so we proceed to Delete.
							return plugin.ReadResponse{ReadResult: plugin.ReadResult{
								ID:      req.ID,
								Inputs:  resource.PropertyMap{"name": resource.NewProperty("in")},
								Outputs: resource.PropertyMap{"name": resource.NewProperty("out")},
							}}, nil
						}
						return tt.reread()
					},
					DeleteF: func(_ context.Context, _ plugin.DeleteRequest) (plugin.DeleteResponse, error) {
						return plugin.DeleteResponse{}, errors.New(deleteFailure)
					},
				},
			})
			cmd.SetArgs([]string{"--stateless", "azure:index:myResource", "delete", "res-1", "--yes"})
			err := cmd.Execute()

			require.Error(t, err)
			assert.ErrorContains(t, err, deleteFailure, "the real failure must reach the caller")
			assert.NotContains(t, strings.ToLower(err.Error()), "was not found",
				"an ambiguous re-read must not be reported as a successful cleanup")
			assert.Equal(t, int32(2), reads.Load(), "exactly one re-read, and no Delete retry")
		})
	}
}
