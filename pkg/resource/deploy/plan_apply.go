// Copyright 2016-2018, Pulumi Corporation.
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

package deploy

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

// Options controls the planning and deployment process.
type Options struct {
	Events   Events // an optional events callback interface.
	Parallel int    // the degree of parallelism for resource operations (<=1 for serial).
}

// Events is an interface that can be used to hook interesting engine/planning events.
type Events interface {
	OnResourceStepPre(step Step) (interface{}, error)
	OnResourceStepPost(ctx interface{}, step Step, status resource.Status, err error) error
	OnResourceOutputs(step Step) error
}

// Start initializes and returns an iterator that can be used to step through a plan's individual steps.
func (p *Plan) Start(opts Options) (*PlanIterator, error) {
	// Ask the source for its iterator.
	src, err := p.source.Iterate(opts)
	if err != nil {
		return nil, err
	}

	// Create an iterator that can be used to perform the planning process.
	return &PlanIterator{
		p:           p,
		opts:        opts,
		src:         src,
		stepGen:     newStepGenerator(p, opts),
		pendingNews: make(map[resource.URN]Step),
		dones:       make(map[*resource.State]bool),
	}, nil
}

// PlanSummary is an interface for summarizing the progress of a plan.
type PlanSummary interface {
	Steps() int
	Creates() map[resource.URN]bool
	Updates() map[resource.URN]bool
	Replaces() map[resource.URN]bool
	Deletes() map[resource.URN]bool
	Sames() map[resource.URN]bool
	Resources() []*resource.State
}

// PlanIterator can be used to step through and/or execute a plan's proposed actions.
type PlanIterator struct {
	p       *Plan          // the plan to which this iterator belongs.
	opts    Options        // the options this iterator was created with.
	src     SourceIterator // the iterator that fetches source resources.
	stepGen *stepGenerator // the step generator for this plan.

	pendingNews map[resource.URN]Step // a map of logical steps currently active.

	stepqueue []Step                   // a queue of steps to drain.
	delqueue  []Step                   // a queue of deletes left to perform.
	resources []*resource.State        // the resulting ordered resource states.
	dones     map[*resource.State]bool // true for each old state we're done with.

	srcdone bool // true if the source interpreter has been run to completion.
	done    bool // true if the planning and associated iteration has finished.
}

func (iter *PlanIterator) Plan() *Plan { return iter.p }
func (iter *PlanIterator) Steps() int {
	return len(iter.Creates()) + len(iter.Updates()) + len(iter.Replaces()) + len(iter.Deletes())
}
func (iter *PlanIterator) Creates() map[resource.URN]bool  { return iter.stepGen.Creates() }
func (iter *PlanIterator) Updates() map[resource.URN]bool  { return iter.stepGen.Updates() }
func (iter *PlanIterator) Replaces() map[resource.URN]bool { return iter.stepGen.Replaces() }
func (iter *PlanIterator) Deletes() map[resource.URN]bool  { return iter.stepGen.Deletes() }
func (iter *PlanIterator) Sames() map[resource.URN]bool    { return iter.stepGen.Sames() }
func (iter *PlanIterator) Resources() []*resource.State    { return iter.resources }
func (iter *PlanIterator) Dones() map[*resource.State]bool { return iter.dones }
func (iter *PlanIterator) Done() bool                      { return iter.done }

// Apply performs a plan's step and records its result in the iterator's state.
func (iter *PlanIterator) Apply(step Step, preview bool) (resource.Status, error) {
	urn := step.URN()

	// If there is a pre-event, raise it.
	var eventctx interface{}
	if e := iter.opts.Events; e != nil {
		var eventerr error
		eventctx, eventerr = e.OnResourceStepPre(step)
		if eventerr != nil {
			return resource.StatusOK, errors.Wrapf(eventerr, "pre-step event returned an error")
		}
	}

	// Apply the step.
	logging.V(9).Infof("Applying step %v on %v (preview %v)", step.Op(), urn, preview)
	status, err := step.Apply(preview)

	// If there is no error, proceed to save the state; otherwise, go straight to the exit codepath.
	if err == nil {
		// If we have a state object, and this is a create or update, remember it, as we may need to update it later.
		if step.Logical() && step.New() != nil {
			if prior, has := iter.pendingNews[urn]; has {
				return resource.StatusOK,
					errors.Errorf("resource '%s' registered twice (%s and %s)", urn, prior.Op(), step.Op())
			}

			iter.pendingNews[urn] = step
		}
	}

	// If there is a post-event, raise it, and in any case, return the results.
	if e := iter.opts.Events; e != nil {
		if eventerr := e.OnResourceStepPost(eventctx, step, status, err); eventerr != nil {
			return status, errors.Wrapf(eventerr, "post-step event returned an error")
		}
	}

	// At this point, if err is not nil, we've already issued an error message through our
	// diag subsystem and we need to bail.
	//
	// This error message is ultimately what's going to be presented to the user at the top
	// level, so the message here is intentionally vague; we should have already presented
	// a more specific error message.
	if err != nil {
		if preview {
			return status, errors.New("preview failed")
		}

		return status, errors.New("update failed")
	}

	return status, nil
}

// Close terminates the iteration of this plan.
func (iter *PlanIterator) Close() error {
	return iter.src.Close()
}

// Next advances the plan by a single step, and returns the next step to be performed.  In doing so, it will perform
// evaluation of the program as much as necessary to determine the next step.  If there is no further action to be
// taken, Next will return a nil step pointer.
func (iter *PlanIterator) Next() (Step, error) {
outer:
	for !iter.done {
		if len(iter.stepqueue) > 0 {
			step := iter.stepqueue[0]
			iter.stepqueue = iter.stepqueue[1:]
			return step, nil
		} else if !iter.srcdone {
			event, err := iter.src.Next()
			if err != nil {
				return nil, err
			} else if event != nil {
				// If we have an event, drive the behavior based on which kind it is.
				switch e := event.(type) {
				case RegisterResourceEvent:
					// If the intent is to register a resource, compute the plan steps necessary to do so.
					steps, steperr := iter.stepGen.GenerateSteps(e)
					if steperr != nil {
						return nil, steperr
					}
					contract.Assert(len(steps) > 0)
					if len(steps) > 1 {
						iter.stepqueue = steps[1:]
					}
					return steps[0], nil
				case RegisterResourceOutputsEvent:
					// If the intent is to complete a prior resource registration, do so.  We do this by just
					// processing the request from the existing state, and do not expose our callers to it.
					if err := iter.registerResourceOutputs(e); err != nil {
						return nil, err
					}
					continue outer
				case ReadResourceEvent:
					steps, steperr := iter.stepGen.GenerateReadSteps(e)
					if steperr != nil {
						return nil, steperr
					}

					contract.Assert(len(steps) > 0)
					if len(steps) > 1 {
						iter.stepqueue = steps[1:]
					}
					return steps[0], nil
				default:
					contract.Failf("Unrecognized intent from source iterator: %v", reflect.TypeOf(event))
				}
			}

			// If all returns are nil, the source is done, note it, and don't go back for more.  Add any deletions to be
			// performed, and then keep going 'round the next iteration of the loop so we can wrap up the planning.
			iter.srcdone = true
			iter.delqueue = iter.stepGen.GenerateDeletes()
		} else {
			// The interpreter has finished, so we need to now drain any deletions that piled up.
			if step := iter.nextDeleteStep(); step != nil {
				return step, nil
			}

			// Otherwise, if the deletes have quiesced, there is nothing remaining in this plan; leave.
			iter.done = true
			break
		}
	}
	return nil, nil
}

func (iter *PlanIterator) registerResourceOutputs(e RegisterResourceOutputsEvent) error {
	// Look up the final state in the pending registration list.
	urn := e.URN()
	reg, has := iter.pendingNews[urn]
	contract.Assertf(has, "cannot complete a resource '%v' whose registration isn't pending", urn)
	contract.Assertf(reg != nil, "expected a non-nil resource step ('%v')", urn)
	delete(iter.pendingNews, urn)

	// Unconditionally set the resource's outputs to what was provided.  This intentionally overwrites whatever
	// might already be there, since otherwise "deleting" outputs would have no affect.
	outs := e.Outputs()
	logging.V(7).Infof("Registered resource outputs %s: old=#%d, new=#%d", urn, len(reg.New().Outputs), len(outs))
	reg.New().Outputs = e.Outputs()

	// If there is an event subscription for finishing the resource, execute them.
	if e := iter.opts.Events; e != nil {
		if eventerr := e.OnResourceOutputs(reg); eventerr != nil {
			return errors.Wrapf(eventerr, "resource complete event returned an error")
		}
	}

	// Finally, let the language provider know that we're done processing the event.
	e.Done()
	return nil
}

// nextDeleteStep produces a new step that deletes a resource if necessary.
func (iter *PlanIterator) nextDeleteStep() Step {
	if len(iter.delqueue) > 0 {
		del := iter.delqueue[0]
		iter.delqueue = iter.delqueue[1:]
		return del
	}
	return nil
}

// Provider fetches the provider for a given resource type, possibly lazily allocating the plugins for it.  If a
// provider could not be found, or an error occurred while creating it, a non-nil error is returned.
func (iter *PlanIterator) Provider(t tokens.Type) (plugin.Provider, error) {
	pkg := t.Package()
	prov, err := iter.p.Provider(pkg)
	if err != nil {
		return nil, err
	} else if prov == nil {
		return nil, errors.Errorf("could not load resource provider for package '%v' from $PATH", pkg)
	}
	return prov, nil
}
