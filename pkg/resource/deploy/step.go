// Copyright 2016-2023, Pulumi Corporation.
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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// StepCompleteFunc is the type of functions returned from Step.Apply. These
// functions are to be called when the engine has fully retired a step. You
// _should not_ modify the resource state in these functions -- doing so will
// race with the snapshot writing code.
type StepCompleteFunc func() error

// Step is a specification for a deployment operation.
type Step interface {
	// Apply applies this step. It returns the status of the resource after the
	// step application, a function to call to signal that this step has fully
	// completed, and an error, if one occurred while applying the step.
	//
	// The returned StepCompleteFunc, if not nil, must be called after committing
	// the results of this step into the state of the deployment.
	Apply() (resource.Status, StepCompleteFunc, error)

	// the operation performed by this step.
	Op() display.StepOp
	// the resource URN (for before and after).
	URN() resource.URN
	// the type of the resource affected by this step.
	Type() tokens.Type
	// the provider reference for the resource affected by this step.
	Provider() string
	// the state of the resource before performing this step.
	Old() *resource.State
	// the state of the resource after performing this step.
	New() *resource.State
	// the latest state for the resource that is known (worst case, old).
	Res() *resource.State
	// true if this step represents a logical operation in the program.
	Logical() bool
	// the deployment to which this step belongs.
	Deployment() *Deployment

	// Calling Fail will mark the step as failed.
	Fail()
	// Calling Skip will mark the step as skipped.
	Skip()
}

// SameStep is a mutating step that does nothing.
type SameStep struct {
	deployment *Deployment           // the current deployment.
	reg        RegisterResourceEvent // the registration intent to convey a URN back to.
	old        *resource.State       // the state of the resource before this step.
	new        *resource.State       // the state of the resource after this step.

	// If this is a same-step for a resource being created but which was not --target'ed by the user
	// (and thus was skipped).
	skippedCreate bool
}

var _ Step = (*SameStep)(nil)

func NewSameStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State) Step {
	contract.Requiref(old != new, "old and new", "must not be the same")

	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is custom")
	contract.Requiref(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type),
		"old", "must have or be a provider if it is a custom resource")
	contract.Requiref(!old.Delete, "old", "must not be marked for deletion")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type),
		"new", "must have or be a provider if it is a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")

	return &SameStep{
		deployment: deployment,
		reg:        reg,
		old:        old,
		new:        new,
	}
}

// NewSkippedCreateStep produces a SameStep for a resource that was created but not targeted
// by the user (and thus was skipped). These act as no-op steps (hence 'same') since we are not
// actually creating the resource, but ensure that we complete resource-registration and convey the
// right information downstream. For example, we will not write these into the checkpoint file.
func NewSkippedCreateStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State) Step {
	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type),
		"new", "must have or be a provider if it is a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	// If we don't have an old state make the old state here a direct copy of the new state
	old := new.Copy()
	return &SameStep{
		deployment:    deployment,
		reg:           reg,
		old:           old,
		new:           new,
		skippedCreate: true,
	}
}

func (s *SameStep) Op() display.StepOp      { return OpSame }
func (s *SameStep) Deployment() *Deployment { return s.deployment }
func (s *SameStep) Type() tokens.Type       { return s.new.Type }
func (s *SameStep) Provider() string        { return s.new.Provider }
func (s *SameStep) URN() resource.URN       { return s.new.URN }
func (s *SameStep) Old() *resource.State    { return s.old }
func (s *SameStep) New() *resource.State    { return s.new }
func (s *SameStep) Res() *resource.State    { return s.new }
func (s *SameStep) Logical() bool           { return true }

func (s *SameStep) Apply() (resource.Status, StepCompleteFunc, error) {
	s.new.Lock.Lock()
	defer s.new.Lock.Unlock()

	// Retain the ID and outputs
	s.new.ID = s.old.ID
	s.new.Outputs = s.old.Outputs

	// If the resource is a provider, ensure that it is present in the registry under the appropriate URNs.
	// We can only do this if the provider is actually a same, not a skipped create.
	if providers.IsProviderType(s.new.Type) && !s.skippedCreate {
		if s.Deployment() != nil {
			// We need to use the new state here (so that URN and ID are correct), but we want to use the old
			// inputs. This ensures that providers that report changed inputs as NO_DIFF consistently see the
			// old inputs, not the new ones.
			st := s.new.Copy()
			st.Inputs = s.old.Inputs
			err := s.Deployment().SameProvider(st)
			if err != nil {
				return resource.StatusOK, nil,
					fmt.Errorf("bad provider state for resource %v: %w", s.URN(), err)
			}
		}
	}

	// TODO: should this step be marked as skipped if it comes from a targeted up?
	complete := func() error {
		// It's possible that s.reg will be nil in the case that multiple same steps
		// are emitted for a single RegisterResourceEvent. This occurs when a
		// resource which is not targeted by a targeted operation needs to ensure
		// that old dependencies are brought forward into the new state before it
		// is. In these cases the root resource will be instantiated with its event
		// while its dependencies will have a nil event. This is fine since in these
		// cases the only Done callback we care about is the one for the root
		// resource.
		if s.reg != nil {
			s.reg.Done(&RegisterResult{State: s.new})
		}
		return nil
	}
	return resource.StatusOK, complete, nil
}

func (s *SameStep) IsSkippedCreate() bool {
	return s.skippedCreate
}

func (s *SameStep) Fail() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateFailed})
}

func (s *SameStep) Skip() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateSkipped})
}

// CreateStep is a mutating step that creates an entirely new resource.
type CreateStep struct {
	deployment    *Deployment                    // the current deployment.
	reg           RegisterResourceEvent          // the registration intent to convey a URN back to.
	old           *resource.State                // the state of the existing resource (only for replacements).
	new           *resource.State                // the state of the resource after this step.
	keys          []resource.PropertyKey         // the keys causing replacement (only for replacements).
	diffs         []resource.PropertyKey         // the keys causing a diff (only for replacements).
	detailedDiff  map[string]plugin.PropertyDiff // the structured property diff (only for replacements).
	replacing     bool                           // true if this is a create due to a replacement.
	pendingDelete bool                           // true if this replacement should create a pending delete.
	provider      plugin.Provider                // the optional provider to use.
}

var _ Step = (*CreateStep)(nil)

func NewCreateStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State) Step {
	contract.Requiref(reg != nil, "reg", "must not be nil")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type),
		"new", "must have or be a provider if it is a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be external")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &CreateStep{
		deployment: deployment,
		reg:        reg,
		new:        new,
	}
}

func NewCreateReplacementStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State,
	keys, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff, pendingDelete bool,
) Step {
	contract.Requiref(reg != nil, "reg", "must not be nil")

	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is a custom resource")
	contract.Requiref(!old.Delete, "old", "must not be marked for deletion")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type),
		"new", "must have or be a provider if it is a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be external")

	// TODO: Do we want to allow a view to become an actual resource and vice versa? Probably.
	contract.Requiref(old.ViewOf == "", "old", "must not be a view")
	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &CreateStep{
		deployment:    deployment,
		reg:           reg,
		old:           old,
		new:           new,
		keys:          keys,
		diffs:         diffs,
		detailedDiff:  detailedDiff,
		replacing:     true,
		pendingDelete: pendingDelete,
	}
}

func (s *CreateStep) Op() display.StepOp {
	if s.replacing {
		return OpCreateReplacement
	}
	return OpCreate
}
func (s *CreateStep) Deployment() *Deployment                      { return s.deployment }
func (s *CreateStep) Type() tokens.Type                            { return s.new.Type }
func (s *CreateStep) Provider() string                             { return s.new.Provider }
func (s *CreateStep) URN() resource.URN                            { return s.new.URN }
func (s *CreateStep) Old() *resource.State                         { return s.old }
func (s *CreateStep) New() *resource.State                         { return s.new }
func (s *CreateStep) Res() *resource.State                         { return s.new }
func (s *CreateStep) Keys() []resource.PropertyKey                 { return s.keys }
func (s *CreateStep) Diffs() []resource.PropertyKey                { return s.diffs }
func (s *CreateStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.detailedDiff }
func (s *CreateStep) Logical() bool                                { return !s.replacing }

func (s *CreateStep) Apply() (resource.Status, StepCompleteFunc, error) {
	var resourceError error
	resourceStatus := resource.StatusOK

	id := s.new.ID
	outs := s.new.Outputs

	var resourceStatusToken string

	if s.new.Custom {
		// Invoke the Create RPC function for this provider:
		prov, err := getProvider(s, s.provider)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resourceStatusAddress := s.deployment.resourceStatus.Address()
		resourceStatusToken, err = s.deployment.resourceStatus.ReserveToken(s.URN())
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resp, err := prov.Create(context.TODO(), plugin.CreateRequest{
			URN:                   s.URN(),
			Name:                  s.new.URN.Name(),
			Type:                  s.new.URN.Type(),
			Properties:            s.new.Inputs,
			Timeout:               s.new.CustomTimeouts.Create,
			Preview:               s.deployment.opts.DryRun,
			ResourceStatusAddress: resourceStatusAddress,
			ResourceStatusToken:   resourceStatusToken,
		})
		if err != nil {
			if resp.Status != resource.StatusPartialFailure {
				return resp.Status, nil, err
			}

			resourceError = err
			resourceStatus = resp.Status

			if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
				s.new.InitErrors = initErr.Reasons
			}
		}

		id = resp.ID
		outs = resp.Properties

		if !s.deployment.opts.DryRun && id == "" {
			return resourceStatus, nil, errors.New("provider did not return an ID from Create")
		}
	}

	s.new.Lock.Lock()
	defer s.new.Lock.Unlock()

	// Copy any of the default and output properties on the live object state.
	s.new.ID = id
	s.new.Outputs = outs

	// Create should set the Create and Modified timestamps as the resource state has been created.
	now := time.Now().UTC()
	s.new.Created = &now
	s.new.Modified = &now

	// Mark the old resource as pending deletion if necessary.
	if s.replacing && s.pendingDelete {
		contract.Assertf(s.old != s.new, "old and new states should not be the same")
		s.old.Lock.Lock()
		s.old.Delete = true
		s.old.Lock.Unlock()
	}

	complete := func() error {
		err := s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)
		if err != nil {
			return err
		}
		s.reg.Done(&RegisterResult{State: s.new})
		return nil
	}

	if resourceError != nil {
		// If we have a failure, we should return an empty complete function
		// and let the Fail method handle the registration.
		return resourceStatus, nil, resourceError
	}

	return resourceStatus, complete, nil
}

func (s *CreateStep) Fail() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateFailed})
}

func (s *CreateStep) Skip() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateSkipped})
}

// DeleteStep is a mutating step that deletes an existing resource. If `old` is marked "External",
// DeleteStep is a no-op.
type DeleteStep struct {
	deployment         *Deployment           // the current deployment.
	old                *resource.State       // the state of the existing resource.
	pendingReplacement bool                  // true if this resource is pending replacement.
	replacing          bool                  // true if part of a replacement.
	otherDeletions     map[resource.URN]bool // other resources that are planned to delete
	provider           plugin.Provider       // the optional provider to use.
	oldViews           []plugin.View         // the old views for this resource.
}

var _ Step = (*DeleteStep)(nil)

func NewDeleteStep(deployment *Deployment, otherDeletions map[resource.URN]bool, old *resource.State,
	oldViews []plugin.View,
) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is a custom resource")
	contract.Requiref(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type),
		"old", "must have or be a provider if it is a custom resource")
	contract.Requiref(otherDeletions != nil, "otherDeletions", "must not be nil")

	contract.Requiref(old.ViewOf == "", "old", "must not be a view")

	return &DeleteStep{
		deployment:     deployment,
		old:            old,
		otherDeletions: otherDeletions,
		oldViews:       oldViews,
	}
}

func NewDeleteReplacementStep(
	deployment *Deployment,
	otherDeletions map[resource.URN]bool,
	old *resource.State,
	pendingReplace bool,
	oldViews []plugin.View,
) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is a custom resource")
	contract.Requiref(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type),
		"old", "must have or be a provider if it is a custom resource")

	contract.Requiref(otherDeletions != nil, "otherDeletions", "must not be nil")
	contract.Assertf(pendingReplace != old.Delete,
		"resource %v cannot be pending replacement and deletion at the same time", old.URN)

	contract.Requiref(old.ViewOf == "", "old", "must not be a view")

	return &DeleteStep{
		deployment:         deployment,
		otherDeletions:     otherDeletions,
		old:                old,
		pendingReplacement: pendingReplace,
		replacing:          true,
		oldViews:           oldViews,
	}
}

func (s *DeleteStep) Op() display.StepOp {
	if s.old.External {
		if s.replacing {
			return OpDiscardReplaced
		}
		return OpReadDiscard
	}

	if s.replacing {
		return OpDeleteReplaced
	}
	return OpDelete
}
func (s *DeleteStep) Deployment() *Deployment { return s.deployment }
func (s *DeleteStep) Type() tokens.Type       { return s.old.Type }
func (s *DeleteStep) Provider() string        { return s.old.Provider }
func (s *DeleteStep) URN() resource.URN       { return s.old.URN }
func (s *DeleteStep) Old() *resource.State    { return s.old }
func (s *DeleteStep) New() *resource.State    { return nil }
func (s *DeleteStep) Res() *resource.State    { return s.old }
func (s *DeleteStep) Logical() bool           { return !s.replacing }

func isDeletedWith(with resource.URN, otherDeletions map[resource.URN]bool) bool {
	if with == "" {
		return false
	}
	r, ok := otherDeletions[with]
	if !ok {
		return false
	}
	return r
}

type deleteProtectedError struct {
	urn resource.URN
}

func (d deleteProtectedError) Error() string {
	return fmt.Sprintf("resource %[1]q cannot be deleted\n"+
		"because it is protected. To unprotect the resource, "+
		"either remove the `protect` flag from the resource in your Pulumi "+
		"program and run `pulumi up`, or use the command:\n"+
		"`pulumi state unprotect %[2]s`", d.urn, d.urn.Quote())
}

func (s *DeleteStep) Apply() (resource.Status, StepCompleteFunc, error) {
	var resourceStatusToken string

	// Refuse to delete protected resources (unless we're replacing them in
	// which case we will of checked protect elsewhere)
	if !s.replacing && s.old.Protect {
		return resource.StatusOK, nil, deleteProtectedError{urn: s.old.URN}
	}

	if s.deployment.opts.DryRun {
		// Do nothing in preview
	} else if s.old.External {
		// Deleting an External resource is a no-op, since Pulumi does not own the lifecycle.
	} else if s.old.RetainOnDelete {
		// Deleting a "drop on delete" is a no-op as the user has explicitly asked us to not delete the resource.
	} else if isDeletedWith(s.old.DeletedWith, s.otherDeletions) {
		// No need to delete this resource since this resource will be deleted by the another deletion
	} else if s.old.Custom {
		// Not preview and not external and not Drop and is custom, do the actual delete

		// Invoke the Delete RPC function for this provider:
		prov, err := getProvider(s, s.provider)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resourceStatusAddress := s.deployment.resourceStatus.Address()
		resourceStatusToken, err = s.deployment.resourceStatus.ReserveToken(s.URN())
		if err != nil {
			return resource.StatusOK, nil, err
		}

		if rst, err := prov.Delete(context.TODO(), plugin.DeleteRequest{
			URN:                   s.URN(),
			Name:                  s.URN().Name(),
			Type:                  s.URN().Type(),
			ID:                    s.old.ID,
			Inputs:                s.old.Inputs,
			Outputs:               s.old.Outputs,
			Timeout:               s.old.CustomTimeouts.Delete,
			ResourceStatusAddress: resourceStatusAddress,
			ResourceStatusToken:   resourceStatusToken,
			OldViews:              s.oldViews,
		}); err != nil {
			return rst.Status, nil, err
		}
	}

	// Delete steps may occur as part of a replacement chain (where a
	// replacement is effectively implemented as a combination of a create and a
	// delete). The two combinations (create first, or create last) give us the
	// two kinds of replacements Pulumi currently supports:
	// delete-before-replace and delete-after-replace.
	//
	// In the case of a delete-before-replace, we want the resource to remain in
	// the persistence layer throughout the replace operation, even at the point
	// where the delete has occurred but the create has not (yet). This is the
	// purpose of `resource.State`'s `PendingReplacement` field, which we set
	// here.
	//
	// Note: we _don't_ want to set this before the provider delete call (if we
	// need to perform one). The call could fail, and having `PendingReplacement`
	// set in that case would be problematic. `PendingReplacement`'s purpose is to
	// indicate that a delete has already been performed and that an appropriate
	// create needs to be carried out to complete the operation. If an operation
	// is interrupted but `PendingReplacement` is set, the delete will not be
	// retried on the next operation. If the delete failed, we _do_ want to retry
	// it, so only set `PendingReplacement` after we know it's succeeded.
	if s.pendingReplacement {
		s.old.Lock.Lock()
		s.old.PendingReplacement = true
		s.old.Lock.Unlock()
	}

	complete := func() error {
		return s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)
	}

	return resource.StatusOK, complete, nil
}

func (s *DeleteStep) Fail() {
	// Nothing to do here.
}

func (s *DeleteStep) Skip() {
	// Nothing to do here.
}

type RemovePendingReplaceStep struct {
	deployment *Deployment     // the current deployment.
	old        *resource.State // the state of the existing resource.
}

func NewRemovePendingReplaceStep(deployment *Deployment, old *resource.State) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.PendingReplacement, "old", "must be pending replacement")

	contract.Requiref(old.ViewOf == "", "old", "must not be a view")

	return &RemovePendingReplaceStep{
		deployment: deployment,
		old:        old,
	}
}

func (s *RemovePendingReplaceStep) Op() display.StepOp {
	return OpRemovePendingReplace
}
func (s *RemovePendingReplaceStep) Deployment() *Deployment { return s.deployment }
func (s *RemovePendingReplaceStep) Type() tokens.Type       { return s.old.Type }
func (s *RemovePendingReplaceStep) Provider() string        { return s.old.Provider }
func (s *RemovePendingReplaceStep) URN() resource.URN       { return s.old.URN }
func (s *RemovePendingReplaceStep) Old() *resource.State    { return s.old }
func (s *RemovePendingReplaceStep) New() *resource.State    { return nil }
func (s *RemovePendingReplaceStep) Res() *resource.State    { return s.old }
func (s *RemovePendingReplaceStep) Logical() bool           { return false }

func (s *RemovePendingReplaceStep) Apply() (resource.Status, StepCompleteFunc, error) {
	return resource.StatusOK, nil, nil
}

func (s *RemovePendingReplaceStep) Fail() {
	// Nothing to do here.
}

func (s *RemovePendingReplaceStep) Skip() {
	// Nothing to do here.
}

// UpdateStep is a mutating step that updates an existing resource's state.
type UpdateStep struct {
	deployment    *Deployment                    // the current deployment.
	reg           RegisterResourceEvent          // the registration intent to convey a URN back to.
	old           *resource.State                // the state of the existing resource.
	new           *resource.State                // the newly computed state of the resource after updating.
	stables       []resource.PropertyKey         // an optional list of properties that won't change during this update.
	diffs         []resource.PropertyKey         // the keys causing a diff.
	detailedDiff  map[string]plugin.PropertyDiff // the structured diff.
	ignoreChanges []string                       // a list of property paths to ignore when updating.
	provider      plugin.Provider                // the optional provider to use.
	oldViews      []plugin.View                  // the old views for this resource.
}

var _ Step = (*UpdateStep)(nil)

func NewUpdateStep(deployment *Deployment, reg RegisterResourceEvent, old, new *resource.State,
	stables, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff,
	ignoreChanges []string, oldViews []plugin.View,
) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is a custom resource")
	contract.Requiref(!old.Custom || old.Provider != "" || providers.IsProviderType(old.Type),
		"old", "must have or be a provider if it is a custom resource")
	contract.Requiref(!old.Delete, "old", "must not be marked for deletion")
	contract.Requiref(!old.External, "old", "must not be an external resource")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.Provider != "" || providers.IsProviderType(new.Type),
		"new", "must have or be a provider if it is a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be an external resource")

	contract.Requiref(old.ViewOf == "", "old", "must not be a view")
	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &UpdateStep{
		deployment:    deployment,
		reg:           reg,
		old:           old,
		new:           new,
		stables:       stables,
		diffs:         diffs,
		detailedDiff:  detailedDiff,
		ignoreChanges: ignoreChanges,
		oldViews:      oldViews,
	}
}

func (s *UpdateStep) Op() display.StepOp                           { return OpUpdate }
func (s *UpdateStep) Deployment() *Deployment                      { return s.deployment }
func (s *UpdateStep) Type() tokens.Type                            { return s.new.Type }
func (s *UpdateStep) Provider() string                             { return s.new.Provider }
func (s *UpdateStep) URN() resource.URN                            { return s.new.URN }
func (s *UpdateStep) Old() *resource.State                         { return s.old }
func (s *UpdateStep) New() *resource.State                         { return s.new }
func (s *UpdateStep) Res() *resource.State                         { return s.new }
func (s *UpdateStep) Logical() bool                                { return true }
func (s *UpdateStep) Diffs() []resource.PropertyKey                { return s.diffs }
func (s *UpdateStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.detailedDiff }

func (s *UpdateStep) Apply() (resource.Status, StepCompleteFunc, error) {
	// Always propagate the ID and timestamps even in previews and refreshes.
	s.new.Lock.Lock()
	s.new.ID = s.old.ID
	s.new.Created = s.old.Created
	s.new.Modified = s.old.Modified
	s.new.Lock.Unlock()

	var resourceError error
	resourceStatus := resource.StatusOK

	var resourceStatusToken string

	if s.new.Custom {
		// Invoke the Update RPC function for this provider:
		prov, err := getProvider(s, s.provider)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resourceStatusAddress := s.deployment.resourceStatus.Address()
		resourceStatusToken, err = s.deployment.resourceStatus.ReserveToken(s.URN())
		if err != nil {
			return resource.StatusOK, nil, err
		}

		// Update to the combination of the old "all" state, but overwritten with new inputs.
		resp, upderr := prov.Update(context.TODO(), plugin.UpdateRequest{
			URN:                   s.URN(),
			Name:                  s.URN().Name(),
			Type:                  s.URN().Type(),
			ID:                    s.old.ID,
			OldInputs:             s.old.Inputs,
			OldOutputs:            s.old.Outputs,
			NewInputs:             s.new.Inputs,
			Timeout:               s.new.CustomTimeouts.Update,
			IgnoreChanges:         s.ignoreChanges,
			Preview:               s.deployment.opts.DryRun,
			ResourceStatusAddress: resourceStatusAddress,
			ResourceStatusToken:   resourceStatusToken,
			OldViews:              s.oldViews,
		})

		s.new.Lock.Lock()
		defer s.new.Lock.Unlock()

		if upderr != nil {
			if resp.Status != resource.StatusPartialFailure {
				return resp.Status, nil, upderr
			}

			resourceError = upderr
			resourceStatus = resp.Status

			if initErr, isInitErr := upderr.(*plugin.InitError); isInitErr {
				s.new.InitErrors = initErr.Reasons
			}
		}

		// Now copy any output state back in case the update triggered cascading updates to other properties.
		s.new.Outputs = resp.Properties

		// UpdateStep doesn't create, but does modify state.
		// Change the Modified timestamp.
		now := time.Now().UTC()
		s.new.Modified = &now
	}

	// Finally, mark this operation as complete.
	complete := func() error {
		err := s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)
		if err != nil {
			return err
		}
		s.reg.Done(&RegisterResult{State: s.new})
		return nil
	}

	if resourceError != nil {
		// If we have a failure, we should return an empty complete function
		// and let the Fail method handle the registration.
		return resourceStatus, nil, resourceError
	}
	return resourceStatus, complete, nil
}

func (s *UpdateStep) Fail() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateFailed})
}

func (s *UpdateStep) Skip() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateSkipped})
}

// ReplaceStep is a logical step indicating a resource will be replaced.  This is comprised of three physical steps:
// a creation of the new resource, any number of intervening updates of dependents to the new resource, and then
// a deletion of the now-replaced old resource.  This logical step is primarily here for tools and visualization.
type ReplaceStep struct {
	deployment    *Deployment                    // the current deployment.
	old           *resource.State                // the state of the existing resource.
	new           *resource.State                // the new state snapshot.
	keys          []resource.PropertyKey         // the keys causing replacement.
	diffs         []resource.PropertyKey         // the keys causing a diff.
	detailedDiff  map[string]plugin.PropertyDiff // the structured property diff.
	pendingDelete bool                           // true if a pending deletion should happen.
}

var _ Step = (*ReplaceStep)(nil)

func NewReplaceStep(deployment *Deployment, old, new *resource.State, keys, diffs []resource.PropertyKey,
	detailedDiff map[string]plugin.PropertyDiff, pendingDelete bool,
) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.URN != "", "old", "must have a URN")
	contract.Requiref(old.ID != "" || !old.Custom, "old", "must have an ID if it is a custom resource")
	contract.Requiref(!old.Delete, "old", "must not be marked for deletion")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	// contract.Assert(new.ID == "")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")

	// TODO: Do we want to allow a view to become an actual resource and vice versa? Probably.
	contract.Requiref(old.ViewOf == "", "old", "must not be a view")
	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &ReplaceStep{
		deployment:    deployment,
		old:           old,
		new:           new,
		keys:          keys,
		diffs:         diffs,
		detailedDiff:  detailedDiff,
		pendingDelete: pendingDelete,
	}
}

func (s *ReplaceStep) Op() display.StepOp                           { return OpReplace }
func (s *ReplaceStep) Deployment() *Deployment                      { return s.deployment }
func (s *ReplaceStep) Type() tokens.Type                            { return s.new.Type }
func (s *ReplaceStep) Provider() string                             { return s.new.Provider }
func (s *ReplaceStep) URN() resource.URN                            { return s.new.URN }
func (s *ReplaceStep) Old() *resource.State                         { return s.old }
func (s *ReplaceStep) New() *resource.State                         { return s.new }
func (s *ReplaceStep) Res() *resource.State                         { return s.new }
func (s *ReplaceStep) Keys() []resource.PropertyKey                 { return s.keys }
func (s *ReplaceStep) Diffs() []resource.PropertyKey                { return s.diffs }
func (s *ReplaceStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.detailedDiff }
func (s *ReplaceStep) Logical() bool                                { return true }

func (s *ReplaceStep) Apply() (resource.Status, StepCompleteFunc, error) {
	// If this is a pending delete, we should have marked the old resource for deletion in the CreateReplacement step.
	contract.Assertf(!s.pendingDelete || s.old.Delete,
		"old resource %v should be marked for deletion if pending delete", s.old.URN)
	return resource.StatusOK, func() error { return nil }, nil
}

func (s *ReplaceStep) Fail() {
	// Nothing to do here.
}

func (s *ReplaceStep) Skip() {
	// Nothing to do here.
}

// ReadStep is a step indicating that an existing resources will be "read" and projected into the Pulumi object
// model. Resources that are read are marked with the "External" bit which indicates to the engine that it does
// not own this resource's lifeycle.
//
// A resource with a given URN can transition freely between an "external" state and a non-external state. If
// a URN that was previously marked "External" (i.e. was the target of a ReadStep in a previous deployment) is the
// target of a RegisterResource in the next deployment, a CreateReplacement step will be issued to indicate the
// transition from external to owned. If a URN that was previously not marked "External" is the target of a
// ReadResource in the next deployment, a ReadReplacement step will be issued to indicate the transition from owned to
// external.
type ReadStep struct {
	deployment *Deployment       // the deployment that produced this read
	event      ReadResourceEvent // the event that should be signaled upon completion
	old        *resource.State   // the old resource state, if one exists for this urn
	new        *resource.State   // the new resource state, to be used to query the provider
	replacing  bool              // whether or not the new resource is replacing the old resource
	provider   plugin.Provider   // the optional provider to use.
}

// NewReadStep creates a new Read step.
func NewReadStep(deployment *Deployment, event ReadResourceEvent, old, new *resource.State) Step {
	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID != "", "new", "must have an ID")
	contract.Requiref(new.External, "new", "must be marked as external")
	contract.Requiref(new.Custom, "new", "must be a custom resource")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	// If Old was given, it's either an external resource or its ID is equal to the
	// ID that we are preparing to read.
	if old != nil {
		contract.Requiref(old.ID == new.ID || old.External,
			"old", "must have the same ID as new or be external")
		contract.Requiref(old.ViewOf == "", "old", "must not be a view")
	}

	return &ReadStep{
		deployment: deployment,
		event:      event,
		old:        old,
		new:        new,
		replacing:  false,
	}
}

// NewReadReplacementStep creates a new Read step with the `replacing` flag set. When executed,
// it will pend deletion of the "old" resource, which must not be an external resource.
func NewReadReplacementStep(deployment *Deployment, event ReadResourceEvent, old, new *resource.State) Step {
	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID != "", "new", "must have an ID")
	contract.Requiref(new.External, "new", "must be marked as external")
	contract.Requiref(new.Custom, "new", "must be a custom resource")

	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(!old.External, "old", "must not be marked as external")

	// TODO: Do we want to allow a view to become an actual resource and vice versa? Probably.
	contract.Requiref(old.ViewOf == "", "old", "must not be a view")
	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &ReadStep{
		deployment: deployment,
		event:      event,
		old:        old,
		new:        new,
		replacing:  true,
	}
}

func (s *ReadStep) Op() display.StepOp {
	if s.replacing {
		return OpReadReplacement
	}

	return OpRead
}

func (s *ReadStep) Deployment() *Deployment { return s.deployment }
func (s *ReadStep) Type() tokens.Type       { return s.new.Type }
func (s *ReadStep) Provider() string        { return s.new.Provider }
func (s *ReadStep) URN() resource.URN       { return s.new.URN }
func (s *ReadStep) Old() *resource.State    { return s.old }
func (s *ReadStep) New() *resource.State    { return s.new }
func (s *ReadStep) Res() *resource.State    { return s.new }
func (s *ReadStep) Logical() bool           { return !s.replacing }

func (s *ReadStep) Apply() (resource.Status, StepCompleteFunc, error) {
	urn := s.new.URN
	id := s.new.ID

	var resourceError error
	resourceStatus := resource.StatusOK

	var resourceStatusToken string

	// Unlike most steps, Read steps run during previews. The only time
	// we can't run is if the ID we are given is unknown.
	if id == plugin.UnknownStringValue {
		s.new.Lock.Lock()
		defer s.new.Lock.Unlock()

		s.new.Outputs = resource.PropertyMap{}
	} else {
		prov, err := getProvider(s, s.provider)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resourceStatusAddress := s.deployment.resourceStatus.Address()
		resourceStatusToken, err = s.deployment.resourceStatus.ReserveToken(s.URN())
		if err != nil {
			return resource.StatusOK, nil, err
		}

		// Technically the only data we have at this point is "inputs", but we've been passing that as "state" to
		// providers since forever and it would probably break things to stop sending that now. Thus this strange double
		// send of inputs as both "inputs" and "state". Something to break to tidy up in V4.
		result, err := prov.Read(context.TODO(), plugin.ReadRequest{
			URN:                   urn,
			Name:                  urn.Name(),
			Type:                  urn.Type(),
			ID:                    id,
			Inputs:                s.new.Inputs,
			State:                 s.new.Inputs,
			ResourceStatusAddress: resourceStatusAddress,
			ResourceStatusToken:   resourceStatusToken,
		})

		s.new.Lock.Lock()
		defer s.new.Lock.Unlock()

		if err != nil {
			if result.Status != resource.StatusPartialFailure {
				return result.Status, nil, err
			}

			resourceError = err
			resourceStatus = result.Status

			if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
				s.new.InitErrors = initErr.Reasons
			}
		}

		// If there is no such resource, return an error indicating as such.
		if result.Outputs == nil {
			return resource.StatusOK, nil, fmt.Errorf("resource '%s' does not exist", id)
		}
		s.new.Outputs = result.Outputs

		if result.ID != "" {
			s.new.ID = result.ID
		}
	}

	// If we were asked to replace an existing, non-External resource, pend the
	// deletion here.
	if s.replacing {
		s.old.Delete = true
	}
	// Propagate timestamps on Read.
	if s.old != nil {
		s.new.Created = s.old.Created
		s.new.Modified = s.old.Modified
	}
	var inputsChange, outputsChange bool
	if s.old != nil {
		inputsChange = !s.new.Inputs.DeepEquals(s.old.Inputs)
		outputsChange = !s.new.Outputs.DeepEquals(s.old.Outputs)
	}
	// Only update the Modified timestamp if read provides new values that differ
	// from the old state.
	if inputsChange || outputsChange {
		now := time.Now().UTC()
		s.new.Modified = &now
	}

	complete := func() error {
		err := s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)
		if err != nil {
			return err
		}
		s.event.Done(&ReadResult{State: s.new})
		return nil
	}

	if resourceError == nil {
		return resourceStatus, complete, nil
	}
	return resourceStatus, complete, resourceError
}

func (s *ReadStep) Fail() {
	s.event.Done(&ReadResult{State: s.new, Result: ResultStateFailed})
}

func (s *ReadStep) Skip() {
	s.event.Done(&ReadResult{State: s.new, Result: ResultStateSkipped})
}

// RefreshStep is a step used to track the progress of a refresh operation. A refresh operation updates the an existing
// resource by reading its current state from its provider plugin. These steps are not issued by the step generator;
// instead, they are issued by the deployment executor as the optional first step in deployment execution.
type RefreshStep struct {
	deployment *Deployment                                // the deployment that produced this refresh
	old        *resource.State                            // the old resource state, if one exists for this urn
	new        *resource.State                            // the new resource state, to be used to query the provider
	provider   plugin.Provider                            // the optional provider to use.
	diff       plugin.DiffResult                          // the diff between the cloud provider and the state file
	cts        *promise.CompletionSource[*resource.State] // the completion source to signal when the refresh is complete
	oldViews   []plugin.View                              // the old views for this resource.
}

// NewRefreshStep creates a new Refresh step.
func NewRefreshStep(deployment *Deployment, cts *promise.CompletionSource[*resource.State], old *resource.State,
	oldViews []plugin.View,
) Step {
	contract.Requiref(old != nil, "old", "must not be nil")
	contract.Requiref(old.ViewOf == "", "old", "must not be a view")

	// NOTE: we set the new state to the old state by default so that we don't interpret step failures as deletes.
	return &RefreshStep{
		deployment: deployment,
		old:        old,
		new:        old,
		cts:        cts,
		oldViews:   oldViews,
	}
}

// True if this is a persisted refresh step that should be respected by the snapshot system.
func (s *RefreshStep) Persisted() bool { return s.cts != nil }

func (s *RefreshStep) Op() display.StepOp                           { return OpRefresh }
func (s *RefreshStep) Deployment() *Deployment                      { return s.deployment }
func (s *RefreshStep) Type() tokens.Type                            { return s.old.Type }
func (s *RefreshStep) Provider() string                             { return s.old.Provider }
func (s *RefreshStep) URN() resource.URN                            { return s.old.URN }
func (s *RefreshStep) Old() *resource.State                         { return s.old }
func (s *RefreshStep) New() *resource.State                         { return s.new }
func (s *RefreshStep) Res() *resource.State                         { return s.old }
func (s *RefreshStep) Logical() bool                                { return false }
func (s *RefreshStep) Diffs() []resource.PropertyKey                { return s.diff.ChangedKeys }
func (s *RefreshStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.diff.DetailedDiff }

// ResultOp returns the operation that corresponds to the change to this resource after reading its current state, if
// any.
func (s *RefreshStep) ResultOp() display.StepOp {
	if s.new == nil {
		return OpDelete
	}

	// There are two cases in which we'll diff only resource outputs on a
	// refresh:
	//
	// * The resource is external, that is, it is not managed by Pulumi.
	//   In these cases, we care somewhat equally about inputs and outputs, but
	//   the Diff contract we currently support forces us to bias one side
	//   (typically inputs). Moreover, some providers might not support
	//   diff/handle diffing correctly for external resources. This can lead to
	//   surprising results, so for now we sidestep the issue by only looking at
	//   outputs.
	// * The user has explicitly opted into this legacy behaviour by setting
	//   the `UseLegacyRefreshDiff` option to true.
	if s.old.External || s.deployment.opts.UseLegacyRefreshDiff {
		if s.new == s.old || s.old.Outputs.Diff(s.new.Outputs) == nil {
			return OpSame
		}
	} else {
		if s.new == s.old || s.diff.Changes == plugin.DiffNone {
			return OpSame
		}
	}

	return OpUpdate
}

func (s *RefreshStep) Apply() (resource.Status, StepCompleteFunc, error) {
	resourceID := s.old.ID

	// Component, provider, and pending-replace resources never change with a refresh; just return the current state.
	if !s.old.Custom || providers.IsProviderType(s.old.Type) || s.old.PendingReplacement {
		return resource.StatusOK, nil, nil
	}

	// For a custom resource, fetch the resource's provider and read the resource's current state.
	prov, err := getProvider(s, s.provider)
	if err != nil {
		return resource.StatusOK, nil, err
	}

	resourceStatusAddress := s.deployment.resourceStatus.Address()
	resourceStatusToken, err := s.deployment.resourceStatus.ReserveToken(s.URN())
	if err != nil {
		return resource.StatusOK, nil, err
	}

	var initErrors []string
	refreshed, err := prov.Read(context.TODO(), plugin.ReadRequest{
		URN:                   s.old.URN,
		Name:                  s.old.URN.Name(),
		Type:                  s.old.URN.Type(),
		ID:                    resourceID,
		Inputs:                s.old.Inputs,
		State:                 s.old.Outputs,
		ResourceStatusAddress: resourceStatusAddress,
		ResourceStatusToken:   resourceStatusToken,
		OldViews:              s.oldViews,
	})
	if err != nil {
		if refreshed.Status != resource.StatusPartialFailure {
			return refreshed.Status, nil, err
		}
		if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
			initErrors = initErr.Reasons

			// Partial failure SHOULD NOT cause refresh to fail. Instead:
			//
			// 1. Warn instead that during refresh we noticed the resource has become unhealthy.
			// 2. Make sure the initialization errors are persisted in the state, so that the next
			//    `pulumi up` will surface them to the user.
			err = nil
			msg := "Refreshed resource is in an unhealthy state:\n* " + strings.Join(initErrors, "\n* ")
			s.Deployment().Diag().Warningf(diag.RawMessage(s.URN(), msg))
		}
	}
	outputs := refreshed.Outputs

	// If the provider specified new inputs for this resource, pick them up now. Otherwise, retain the current inputs.
	inputs := s.old.Inputs
	if refreshed.Inputs != nil {
		inputs = refreshed.Inputs
	}

	if outputs != nil {
		// There is a chance that the ID has changed. We want to allow this change to happen
		// it will have changed already in the outputs, but we need to persist this change
		// at a state level because the Id
		if refreshed.ID != "" && refreshed.ID != resourceID {
			logging.V(7).Infof("Refreshing ID; oldId=%s, newId=%s", resourceID, refreshed.ID)
			resourceID = refreshed.ID
		}

		s.new = resource.NewState(s.old.Type, s.old.URN, s.old.Custom, s.old.Delete, resourceID, inputs, outputs,
			s.old.Parent, s.old.Protect, s.old.External, s.old.Dependencies, initErrors, s.old.Provider,
			s.old.PropertyDependencies, s.old.PendingReplacement, s.old.AdditionalSecretOutputs, s.old.Aliases,
			&s.old.CustomTimeouts, s.old.ImportID, s.old.RetainOnDelete, s.old.DeletedWith, s.old.Created, s.old.Modified,
			s.old.SourcePosition, s.old.IgnoreChanges, s.old.ReplaceOnChanges, s.old.ViewOf,
		)
		var inputsChange, outputsChange bool
		if s.old != nil {
			// There are two cases in which we'll diff only resource outputs on a
			// refresh:
			//
			// * The resource is external, that is, it is not managed by Pulumi.
			//   In these cases, we care somewhat equally about inputs and outputs, but
			//   the Diff contract we currently support forces us to bias one side
			//   (typically inputs). Moreover, some providers might not support
			//   diff/handle diffing correctly for external resources. This can lead to
			//   surprising results, so for now we sidestep the issue by only looking at
			//   outputs.
			// * The user has explicitly opted into this legacy behaviour by setting
			//   the `UseLegacyRefreshDiff` option to true.
			if s.old.External || s.deployment.opts.UseLegacyRefreshDiff {
				inputsChange = !refreshed.Inputs.DeepEquals(s.old.Inputs)
				outputsChange = !refreshed.Outputs.DeepEquals(s.old.Outputs)
			} else {
				inputsChange = !inputs.DeepEquals(s.old.Inputs)
				outputsChange = !outputs.DeepEquals(s.old.Outputs)
			}
		}

		// Only update the Modified timestamp if refresh provides new values that differ
		// from the old state.
		if inputsChange || outputsChange {
			// The refresh has identified an incongruence between the provider and state
			// updated the Modified timestamp to track this.
			now := time.Now().UTC()
			s.new.Modified = &now
		}

		if s.old.External {
			logging.V(7).Infof("External resource %s; diffing outputs only", s.URN())
		} else if s.deployment.opts.UseLegacyRefreshDiff {
			logging.V(7).Infof("Refresh diffing disabled; diffing outputs only (%s)", s.URN())
		} else {
			// To compute refresh diffs against the desired state, we compute the diff
			// that a user would see if they immediately ran an `up` operation on a
			// no-change program after this refresh. However, this will return the
			// _opposite_ of what we want, since the `up`'s diff is framed in terms of
			// the program being the source of truth (not the provider). That is, we
			// want to show the user what changes are coming from the outputs into the
			// inputs, not what changes are coming from the inputs into the outputs!
			// We thus invert the diff (changing adds to deletes, and so on) before
			// storing it against the step.
			//
			// Note that to compute the diff in this manner, we pass:
			//
			// * newInputs where oldInputs are expected
			// * newOutputs where oldOutputs are expected
			// * oldInputs where newInputs are expected
			diff, err := diffResource(
				s.new.URN, s.new.ID,
				// pass new inputs/outputs as old inputs/outputs
				s.new.Inputs, s.new.Outputs,
				// pass old inputs as new inputs
				s.old.Inputs,
				prov, s.deployment.opts.DryRun, s.old.IgnoreChanges,
			)
			if err != nil {
				return refreshed.Status, nil, err
			}

			s.diff = diff.Invert()
			logging.V(7).Infof("Refresh diff for %s: %v", s.URN(), s.diff)
		}
	} else {
		s.new = nil
	}

	complete := func() error {
		err := s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)

		// s.cts will be empty for refreshes that are just being done on state, rather than via a program.
		if s.cts != nil {
			s.cts.MustFulfill(s.new)
		}

		return err
	}

	return refreshed.Status, complete, err
}

func (s *RefreshStep) Fail() {
	// Nothing to do here.
}

func (s *RefreshStep) Skip() {
	// Nothing to do here.
}

type ImportStep struct {
	deployment    *Deployment                    // the current deployment.
	reg           RegisterResourceEvent          // the registration intent to convey a URN back to.
	original      *resource.State                // the original resource, if this is an import-replace.
	old           *resource.State                // the state of the resource fetched from the provider.
	new           *resource.State                // the newly computed state of the resource after importing.
	replacing     bool                           // true if we are replacing a Pulumi-managed resource.
	planned       bool                           // true if this import is from an import deployment.
	diffs         []resource.PropertyKey         // any keys that differed between the user's program and the actual state.
	detailedDiff  map[string]plugin.PropertyDiff // the structured property diff.
	ignoreChanges []string                       // a list of property paths to ignore when updating.
	randomSeed    []byte                         // the random seed to use for Check.
	provider      plugin.Provider                // the optional provider to use.
}

func NewImportStep(deployment *Deployment, reg RegisterResourceEvent, new *resource.State,
	ignoreChanges []string, randomSeed []byte,
) Step {
	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(new.ImportID != "", "new", "must have an ImportID")
	contract.Requiref(new.Custom, "new", "must be a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be external")
	contract.Requiref(randomSeed != nil, "randomSeed", "must not be nil")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &ImportStep{
		deployment:    deployment,
		reg:           reg,
		new:           new,
		ignoreChanges: ignoreChanges,
		randomSeed:    randomSeed,
	}
}

func NewImportReplacementStep(deployment *Deployment, reg RegisterResourceEvent, original, new *resource.State,
	ignoreChanges []string, randomSeed []byte,
) Step {
	contract.Requiref(original != nil, "original", "must not be nil")

	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(new.ImportID != "", "new", "must have an ImportID")
	contract.Requiref(new.Custom, "new", "must be a custom resource")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be external")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	contract.Requiref(randomSeed != nil, "randomSeed", "must not be nil")

	return &ImportStep{
		deployment:    deployment,
		reg:           reg,
		original:      original,
		new:           new,
		replacing:     true,
		ignoreChanges: ignoreChanges,
		randomSeed:    randomSeed,
	}
}

func newImportDeploymentStep(deployment *Deployment, new *resource.State, randomSeed []byte) Step {
	contract.Requiref(new != nil, "new", "must not be nil")
	contract.Requiref(new.URN != "", "new", "must have a URN")
	contract.Requiref(new.ID == "", "new", "must not have an ID")
	contract.Requiref(!new.Custom || new.ImportID != "", "new", "must have an ImportID")
	contract.Requiref(!new.Delete, "new", "must not be marked for deletion")
	contract.Requiref(!new.External, "new", "must not be external")
	contract.Requiref(!new.Custom || randomSeed != nil, "randomSeed", "must not be nil")

	contract.Requiref(new.ViewOf == "", "new", "must not be a view")

	return &ImportStep{
		deployment: deployment,
		reg:        noopEvent(0),
		new:        new,
		planned:    true,
		randomSeed: randomSeed,
	}
}

func (s *ImportStep) Op() display.StepOp {
	if s.replacing {
		return OpImportReplacement
	}
	return OpImport
}

func (s *ImportStep) Deployment() *Deployment                      { return s.deployment }
func (s *ImportStep) Type() tokens.Type                            { return s.new.Type }
func (s *ImportStep) Provider() string                             { return s.new.Provider }
func (s *ImportStep) URN() resource.URN                            { return s.new.URN }
func (s *ImportStep) Old() *resource.State                         { return s.old }
func (s *ImportStep) New() *resource.State                         { return s.new }
func (s *ImportStep) Res() *resource.State                         { return s.new }
func (s *ImportStep) Logical() bool                                { return !s.replacing }
func (s *ImportStep) Diffs() []resource.PropertyKey                { return s.diffs }
func (s *ImportStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.detailedDiff }

func (s *ImportStep) Apply() (resource.Status, StepCompleteFunc, error) {
	// If this is a planned import, ensure that the resource does not exist in the old state file.
	if s.planned {
		if _, ok := s.deployment.olds[s.new.URN]; ok {
			return resource.StatusOK, nil, fmt.Errorf("resource '%v' already exists", s.new.URN)
		}
		if s.new.Parent.QualifiedType() != resource.RootStackType {
			_, ok := s.deployment.news.Load(s.new.Parent)
			if !ok {
				return resource.StatusOK, nil, fmt.Errorf("unknown parent '%v' for resource '%v'",
					s.new.Parent, s.new.URN)
			}
		}
	}

	// Only need to do anything here for custom resources, components just import as empty
	inputs := resource.PropertyMap{}
	outputs := resource.PropertyMap{}
	var prov plugin.Provider
	var resourceStatusToken string
	rst := resource.StatusOK
	if s.new.Custom {
		// Read the current state of the resource to import. If the provider does not hand us back any inputs for the
		// resource, it probably needs to be updated. If the resource does not exist at all, fail the import.
		var err error
		prov, err = getProvider(s, s.provider)
		if err != nil {
			return resource.StatusOK, nil, err
		}

		resourceStatusAddress := s.deployment.resourceStatus.Address()
		resourceStatusToken, err := s.deployment.resourceStatus.ReserveToken(s.URN())
		if err != nil {
			return resource.StatusOK, nil, err
		}

		read, err := prov.Read(context.TODO(), plugin.ReadRequest{
			URN:                   s.new.URN,
			Name:                  s.new.URN.Name(),
			Type:                  s.new.URN.Type(),
			ID:                    s.new.ImportID,
			ResourceStatusAddress: resourceStatusAddress,
			ResourceStatusToken:   resourceStatusToken,
		})
		rst = read.Status

		s.new.Lock.Lock()
		defer s.new.Lock.Unlock()

		if err != nil {
			if initErr, isInitErr := err.(*plugin.InitError); isInitErr {
				s.new.InitErrors = initErr.Reasons
			} else {
				return rst, nil, err
			}
		}
		if read.Outputs == nil {
			return rst, nil, fmt.Errorf("resource '%v' does not exist", s.new.ID)
		}
		if read.Inputs == nil {
			return resource.StatusOK, nil,
				fmt.Errorf("provider does not support importing resources; please try updating the '%v' plugin",
					s.new.URN.Type().Package())
		}
		if read.ID != "" {
			s.new.ID = read.ID
		} else {
			s.new.ID = s.new.ImportID
		}
		inputs = read.Inputs
		outputs = read.Outputs
	} else {
		s.new.Lock.Lock()
		defer s.new.Lock.Unlock()
	}

	s.new.Outputs = outputs
	// Magic up an old state so the frontend can display a proper diff. This state is the output of the just-executed
	// `Read` combined with the resource identity and metadata from the desired state. This ensures that the only
	// differences between the old and new states are between the inputs and outputs.
	s.old = resource.NewState(s.new.Type, s.new.URN, s.new.Custom, false, s.new.ID, inputs, outputs,
		s.new.Parent, s.new.Protect, false, s.new.Dependencies, s.new.InitErrors, s.new.Provider,
		s.new.PropertyDependencies, false, nil, nil, &s.new.CustomTimeouts, s.new.ImportID, s.new.RetainOnDelete,
		s.new.DeletedWith, nil, nil, s.new.SourcePosition, s.new.IgnoreChanges, s.new.ReplaceOnChanges, s.new.ViewOf)

	// Import takes a resource that Pulumi did not create and imports it into pulumi state.
	now := time.Now().UTC()
	s.new.Modified = &now
	// Set Created to now as the resource has been created in the state.
	s.new.Created = &now

	complete := func() error {
		err := s.deployment.resourceStatus.ReleaseToken(resourceStatusToken)
		if err != nil {
			return err
		}
		s.reg.Done(&RegisterResult{State: s.new})
		return nil
	}

	// If this is a component we don't need to do the rest of the input validation
	if !s.new.Custom {
		return rst, complete, nil
	}

	// If this step came from an import deployment, we need to fetch any required inputs from the state.
	if s.planned {
		contract.Assertf(len(s.new.Inputs) == 0, "import resource cannot have existing inputs")

		// Historically, we would never set ImportID for resources imported via `pulumi import`. This
		// continues that behavior. When adding support for https://github.com/pulumi/pulumi/issues/8836,
		// we'll likly need to make this toggleable.
		s.new.ImportID = ""

		// Get the import object and see if it had properties set
		var inputProperties []string
		for _, imp := range s.deployment.imports {
			if imp.ID == s.old.ImportID {
				inputProperties = imp.Properties
				break
			}
		}

		if len(inputProperties) == 0 {
			logging.V(9).Infof("Importing %v with all properties", s.URN())
			s.new.Inputs = s.old.Inputs.Copy()
		} else {
			logging.V(9).Infof("Importing %v with supplied properties: %v", s.URN(), inputProperties)
			for _, p := range inputProperties {
				k := resource.PropertyKey(p)
				if value, has := s.old.Inputs[k]; has {
					s.new.Inputs[k] = value
				}
			}
		}

		// Check the provider inputs for consistency. If the inputs fail validation, the import will still succeed, but
		// we will display the validation failures and a message informing the user that the failures are almost
		// definitely a provider bug.
		resp, err := prov.Check(context.TODO(), plugin.CheckRequest{
			URN:           s.new.URN,
			Name:          s.URN().Name(),
			Type:          s.URN().Type(),
			Olds:          s.old.Inputs,
			News:          s.new.Inputs,
			AllowUnknowns: s.deployment.opts.DryRun,
			RandomSeed:    s.randomSeed,
		})
		if err != nil {
			return rst, nil, err
		}

		// Print this warning before printing all the check failures to give better context.
		if len(resp.Failures) != 0 {
			// Based on if the user passed 'properties' or not we want to change the error message here.
			var errorMessage string
			if len(inputProperties) == 0 {
				ref, err := providers.ParseReference(s.Provider())
				contract.AssertNoErrorf(err, "failed to parse provider reference %q", s.Provider())

				pkgName := ref.URN().Type().Name()
				errorMessage = fmt.Sprintf("This is almost certainly a bug in the `%s` provider.", pkgName)
			} else {
				errorMessage = "Try specifying a different set of properties to import with in the future."
			}

			s.deployment.Diag().Warningf(diag.Message(s.new.URN,
				"One or more imported inputs failed to validate. %s "+
					"The import will still proceed, but you will need to edit the generated code after copying it into your program."),
				errorMessage)
		}

		issueCheckFailures(s.deployment.Diag().Warningf, s.new, s.new.URN, resp.Failures)

		s.diffs, s.detailedDiff = []resource.PropertyKey{}, map[string]plugin.PropertyDiff{}

		return rst, complete, nil
	}

	// Set inputs back to their old values (if any) for any "ignored" properties
	processedInputs, err := processIgnoreChanges(s.new.Inputs, s.old.Inputs, s.ignoreChanges)
	if err != nil {
		return resource.StatusOK, nil, err
	}
	s.new.Inputs = processedInputs

	// Check the inputs using the provider inputs for defaults.
	resp, err := prov.Check(context.TODO(), plugin.CheckRequest{
		URN:           s.new.URN,
		Name:          s.new.URN.Name(),
		Type:          s.new.URN.Type(),
		Olds:          s.old.Inputs,
		News:          s.new.Inputs,
		AllowUnknowns: s.deployment.opts.DryRun,
		RandomSeed:    s.randomSeed,
	})
	if err != nil {
		return rst, nil, err
	}
	if issueCheckErrors(s.deployment, s.new, s.new.URN, resp.Failures) {
		return rst, nil, errors.New("one or more inputs failed to validate")
	}
	s.new.Inputs = resp.Properties

	// Diff the user inputs against the provider inputs. If there are any differences, fail the import unless this step
	// is from an import deployment.
	diff, err := diffResource(
		s.new.URN, s.new.ID,
		s.old.Inputs, s.old.Outputs,
		s.new.Inputs,
		prov,
		s.deployment.opts.DryRun,
		s.ignoreChanges,
	)
	if err != nil {
		return rst, nil, err
	}

	s.diffs, s.detailedDiff = diff.ChangedKeys, diff.DetailedDiff

	if diff.Changes != plugin.DiffNone {
		message := fmt.Sprintf("inputs to import do not match the existing resource: %v", s.diffs)

		if s.deployment.opts.DryRun {
			s.deployment.ctx.Diag.Warningf(diag.StreamMessage(s.new.URN,
				message+"; importing this resource will fail", 0))
		} else {
			err = errors.New(message)
		}
	}

	// If we were asked to replace an existing, non-External resource, pend the deletion here.
	if err == nil && s.replacing {
		s.original.Delete = true
	}

	if err != nil {
		return rst, nil, err
	}
	return rst, complete, nil
}

func (s *ImportStep) Fail() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateFailed})
}

func (s *ImportStep) Skip() {
	s.reg.Done(&RegisterResult{State: s.new, Result: ResultStateSkipped})
}

const (
	OpSame                 display.StepOp = "same"                   // nothing to do.
	OpCreate               display.StepOp = "create"                 // creating a new resource.
	OpUpdate               display.StepOp = "update"                 // updating an existing resource.
	OpDelete               display.StepOp = "delete"                 // deleting an existing resource.
	OpReplace              display.StepOp = "replace"                // replacing a resource with a new one.
	OpCreateReplacement    display.StepOp = "create-replacement"     // creating a new resource for a replacement.
	OpDeleteReplaced       display.StepOp = "delete-replaced"        // deleting an existing resource after replacement.
	OpRead                 display.StepOp = "read"                   // reading an existing resource.
	OpReadReplacement      display.StepOp = "read-replacement"       // reading an existing resource for a replacement.
	OpRefresh              display.StepOp = "refresh"                // refreshing an existing resource.
	OpReadDiscard          display.StepOp = "discard"                // removing a resource that was read.
	OpDiscardReplaced      display.StepOp = "discard-replaced"       // discarding a read resource that was replaced.
	OpRemovePendingReplace display.StepOp = "remove-pending-replace" // removing a pending replace resource.
	OpImport               display.StepOp = "import"                 // import an existing resource.
	OpImportReplacement    display.StepOp = "import-replacement"     // replace an existing resource
	OpDiff                 display.StepOp = "diff"                   // diffing a resource
	// with an imported resource.
)

// StepOps contains the full set of step operation types.
var StepOps = []display.StepOp{
	OpSame,
	OpCreate,
	OpUpdate,
	OpDelete,
	OpReplace,
	OpCreateReplacement,
	OpDeleteReplaced,
	OpRead,
	OpReadReplacement,
	OpRefresh,
	OpReadDiscard,
	OpDiscardReplaced,
	OpRemovePendingReplace,
	OpImport,
	OpImportReplacement,
	OpDiff,
}

func IsReplacementStep(op display.StepOp) bool {
	if op == OpReplace || op == OpCreateReplacement || op == OpDeleteReplaced ||
		op == OpReadReplacement || op == OpDiscardReplaced || op == OpRemovePendingReplace ||
		op == OpImportReplacement {
		return true
	}
	return false
}

// Color returns a suggested color for lines of this op type.
func Color(op display.StepOp) string {
	switch op {
	case OpSame:
		return colors.SpecUnimportant
	case OpCreate, OpImport:
		return colors.SpecCreate
	case OpDelete:
		return colors.SpecDelete
	case OpUpdate:
		return colors.SpecUpdate
	case OpReplace:
		return colors.SpecReplace
	case OpCreateReplacement:
		return colors.SpecCreateReplacement
	case OpDeleteReplaced:
		return colors.SpecDeleteReplaced
	case OpRead:
		return colors.SpecRead
	case OpReadReplacement, OpImportReplacement:
		return colors.SpecReplace
	case OpRefresh:
		return colors.SpecUpdate
	case OpReadDiscard, OpDiscardReplaced:
		return colors.SpecDelete
	case OpRemovePendingReplace:
		return colors.SpecUnimportant
	default:
		contract.Failf("Unrecognized resource step op: '%v'", op)
		return ""
	}
}

// ColorProgress returns a suggested coloring for lines of this of type which
// are progressing.
func ColorProgress(op display.StepOp) string {
	return colors.Bold + Color(op)
}

// Prefix returns a suggested prefix for lines of this op type.
func Prefix(op display.StepOp, done bool) string {
	var color string
	if done {
		color = Color(op)
	} else {
		color = ColorProgress(op)
	}
	return color + RawPrefix(op)
}

// RawPrefix returns the uncolorized prefix text.
func RawPrefix(op display.StepOp) string {
	switch op {
	case OpSame:
		return "  "
	case OpCreate:
		return "+ "
	case OpDelete:
		return "- "
	case OpUpdate:
		return "~ "
	case OpReplace:
		return "+-"
	case OpCreateReplacement:
		return "++"
	case OpDeleteReplaced:
		return "--"
	case OpRead:
		return "> "
	case OpReadReplacement:
		return ">>"
	case OpRefresh:
		return "~ "
	case OpReadDiscard:
		return "< "
	case OpDiscardReplaced:
		return "<<"
	case OpImport:
		return "= "
	case OpImportReplacement:
		return "=>"
	case OpRemovePendingReplace:
		return "~ "
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

func PastTense(op display.StepOp) string {
	switch op {
	case OpSame, OpCreate, OpReplace, OpCreateReplacement, OpUpdate, OpReadReplacement:
		return string(op) + "d"
	case OpRefresh:
		return "refreshed"
	case OpRead:
		return "read"
	case OpReadDiscard, OpDiscardReplaced:
		return "discarded"
	case OpDelete, OpDeleteReplaced:
		return "deleted"
	case OpImport, OpImportReplacement:
		return "imported"
	default:
		contract.Failf("Unexpected resource step op: %v", op)
		return ""
	}
}

// Suffix returns a suggested suffix for lines of this op type.
func Suffix(op display.StepOp) string {
	switch op {
	case OpCreateReplacement, OpUpdate, OpReplace, OpReadReplacement, OpRefresh, OpImportReplacement:
		return colors.Reset // updates and replacements colorize individual lines; get has none
	}
	return ""
}

// ConstrainedTo returns true if this operation is no more impactful than the constraint.
func ConstrainedTo(op display.StepOp, constraint display.StepOp) bool {
	var allowed []display.StepOp
	switch constraint {
	case OpSame, OpDelete, OpRead, OpReadReplacement, OpRefresh, OpReadDiscard, OpDiscardReplaced,
		OpRemovePendingReplace, OpImport, OpImportReplacement:
		allowed = []display.StepOp{constraint}
	case OpCreate:
		allowed = []display.StepOp{OpSame, OpCreate}
	case OpUpdate:
		allowed = []display.StepOp{OpSame, OpUpdate}
	case OpReplace, OpCreateReplacement, OpDeleteReplaced:
		allowed = []display.StepOp{OpSame, OpUpdate, constraint}
	}
	for _, candidate := range allowed {
		if candidate == op {
			return true
		}
	}
	return false
}

// getProvider fetches the provider for the given step.
func getProvider(s Step, override plugin.Provider) (plugin.Provider, error) {
	if override != nil {
		return override, nil
	}
	if providers.IsProviderType(s.Type()) {
		return s.Deployment().providers, nil
	}
	ref, err := providers.ParseReference(s.Provider())
	if err != nil {
		return nil, fmt.Errorf("bad provider reference '%v' for resource %v: %w", s.Provider(), s.URN(), err)
	}
	if providers.IsDenyDefaultsProvider(ref) {
		pkg := providers.GetDeniedDefaultProviderPkg(ref)
		msg := diag.GetDefaultProviderDenied(s.URN()).Message
		return nil, fmt.Errorf(msg, pkg, s.URN())
	}
	provider, ok := s.Deployment().GetProvider(ref)
	if !ok {
		return nil, fmt.Errorf("unknown provider '%v' for resource %v", s.Provider(), s.URN())
	}
	return provider, nil
}

// DiffStep isn't really a step like a normal step. It's just a way to get access to the parallel but bounded step
// workers. We use this step to call `provider.Diff` in parallel with other steps.
type DiffStep struct {
	deployment    *Deployment                                  // the deployment that produced this diff
	pcs           *promise.CompletionSource[plugin.DiffResult] // the completion source for this diff
	old           *resource.State                              // the old resource state
	new           *resource.State                              // the new resource state
	ignoreChanges []string                                     // a list of property paths to ignore when diffing
}

func NewDiffStep(
	deployment *Deployment, pcs *promise.CompletionSource[plugin.DiffResult], old, new *resource.State,
	ignoreChanges []string,
) Step {
	return &DiffStep{
		deployment:    deployment,
		pcs:           pcs,
		old:           old,
		new:           new,
		ignoreChanges: ignoreChanges,
	}
}

func (s *DiffStep) Op() display.StepOp {
	return OpDiff
}

func (s *DiffStep) Deployment() *Deployment { return s.deployment }
func (s *DiffStep) Type() tokens.Type       { return s.new.Type }
func (s *DiffStep) Provider() string        { return s.new.Provider }
func (s *DiffStep) URN() resource.URN       { return s.new.URN }
func (s *DiffStep) Old() *resource.State    { return s.old }
func (s *DiffStep) New() *resource.State    { return s.new }
func (s *DiffStep) Res() *resource.State    { return s.new }
func (s *DiffStep) Logical() bool           { return true }

func (s *DiffStep) Apply() (resource.Status, StepCompleteFunc, error) {
	// DiffStep is a special step in that we're just using it as a way to get access to the parallel step
	// workers. We don't actually want it to participate in the rest of what normally happens for step
	// execution. As such we never actually return an error here, we just reject the completion source in an
	// error case instead. The step generator will pick that error up and turn it into a stepgen error.

	prov, err := getProvider(s, nil)
	if err != nil {
		s.pcs.Reject(err)
		return resource.StatusOK, nil, nil
	}

	diff, err := diffResource(
		s.new.URN, s.old.ID, s.old.Inputs, s.old.Outputs, s.new.Inputs, prov, s.deployment.opts.DryRun, s.ignoreChanges)
	if err != nil {
		s.pcs.Reject(err)
		return resource.StatusOK, nil, nil
	}
	s.pcs.Fulfill(diff)

	return resource.StatusOK, nil, nil
}

func (s *DiffStep) Fail() {
	s.pcs.Reject(errors.New("failed diff resource"))
}

func (s *DiffStep) Skip() {
	s.pcs.Reject(errors.New("skipped diff resource"))
}

// ViewStep isn't like a normal step. It's a virtual step for a view resource. The step itself
// doesn't perform any operations against a provider, it's used to communicate the steps that
// were taken for the view resource, primarily for display purposes and analysis.
type ViewStep struct {
	deployment   *Deployment                    // the current deployment.
	op           display.StepOp                 // the operation that was performed.
	status       resource.Status                // the status of the operation.
	error        string                         // whether an error occurred (empty if not).
	old          *resource.State                // the state of the existing resource.
	new          *resource.State                // the state of the resource after this step.
	keys         []resource.PropertyKey         // the keys causing replacement (only for replacements).
	diffs        []resource.PropertyKey         // the keys causing a diff (only for replacements).
	detailedDiff map[string]plugin.PropertyDiff // the structured property diff (only for replacements).
}

func NewViewStep(
	deployment *Deployment, op display.StepOp, status resource.Status, err string, old, new *resource.State,
	keys, diffs []resource.PropertyKey, detailedDiff map[string]plugin.PropertyDiff,
) Step {
	return &ViewStep{
		deployment:   deployment,
		op:           op,
		status:       status,
		error:        err,
		old:          old,
		new:          new,
		keys:         keys,
		diffs:        diffs,
		detailedDiff: detailedDiff,
	}
}

func (s *ViewStep) Op() display.StepOp      { return s.op }
func (s *ViewStep) Deployment() *Deployment { return s.deployment }
func (s *ViewStep) Type() tokens.Type       { return s.Res().Type }
func (s *ViewStep) Provider() string        { return s.Res().Provider }
func (s *ViewStep) URN() resource.URN       { return s.Res().URN }
func (s *ViewStep) Old() *resource.State    { return s.old }
func (s *ViewStep) New() *resource.State    { return s.new }
func (s *ViewStep) Res() *resource.State {
	if s.new != nil {
		return s.new
	}
	return s.old
}
func (s *ViewStep) Keys() []resource.PropertyKey                 { return s.keys }
func (s *ViewStep) Diffs() []resource.PropertyKey                { return s.diffs }
func (s *ViewStep) DetailedDiff() map[string]plugin.PropertyDiff { return s.detailedDiff }
func (s *ViewStep) Logical() bool                                { return true }

func (s *ViewStep) Apply() (resource.Status, StepCompleteFunc, error) {
	// ViewStep is a special step that that represents an operation for a view resource.
	// It doesn't actually do anything in Apply. It's used to flow the step through the
	// system for display in the UI and so the the result of the operation is recorded
	// in the state.

	if s.error != "" {
		return s.status, nil, errors.New(s.error)
	}
	return s.status, nil, nil
}

func (s *ViewStep) Fail() {
	// Nothing to do here.
}

func (s *ViewStep) Skip() {
	// Nothing to do here.
}
