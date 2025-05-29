// Copyright 2025, Pulumi Corporation.
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
	"sync"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

type resourceStatusServer struct {
	pulumirpc.UnsafeResourceStatusServer

	// The deployment to which this step generator belongs.
	deployment *Deployment

	// The step executor owned by this deployment.
	stepExec *stepExecutor

	// The address the server is listening on.
	address string

	// A map of tokens to URNs.
	tokens gsync.Map[string, *tokenInfo] // token -> tokenInfo
}

type tokenInfo struct {
	// The owning resource URN for this token.
	urn resource.URN

	// Whether this token is for a refresh operation.
	refresh bool

	// Protects the steps slice.
	mu sync.Mutex

	// Steps that were published with this token.
	steps []stepPre
}

type stepPre struct {
	step    Step
	payload interface{}
}

func newResourceStatusServer(deployment *Deployment, stepExec *stepExecutor) (*resourceStatusServer, error) {
	rs := &resourceStatusServer{
		deployment: deployment,
		stepExec:   stepExec,
		tokens:     gsync.Map[string, *tokenInfo]{},
	}

	// Fire up a gRPC server and start listening for incomings.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceStatusServer(srv, rs)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	rs.address = fmt.Sprintf("127.0.0.1:%d", handle.Port)

	return rs, nil
}

func (rs *resourceStatusServer) Address() string {
	return rs.address
}

func (rs *resourceStatusServer) ReserveToken(urn resource.URN, refresh bool) (string, error) {
	token, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("creating token: %w", err)
	}
	tokenString := token.String()
	rs.tokens.Store(tokenString, &tokenInfo{
		urn:     urn,
		refresh: refresh,
	})
	return tokenString, nil
}

func (rs *resourceStatusServer) ReleaseToken(token string) error {
	if token == "" {
		return nil
	}

	info, ok := rs.tokens.LoadAndDelete(token)
	if !ok {
		return nil
	}

	info.mu.Lock()
	steps := info.steps
	info.mu.Unlock()

	// Execute the steps in the order they were published.
	for _, step := range steps {
		var saf StepApplyFailed
		err := rs.stepExec.continueExecuteStep(step.payload, -1, step.step)
		// We don't need to handle StepApplyFailed errors, they are handled by the
		// step executor when recording the OnResourceStepPost event.
		if err != nil && !errors.As(err, &saf) {
			return err
		}
	}
	return nil
}

func (rs *resourceStatusServer) PublishViewSteps(ctx context.Context,
	req *pulumirpc.PublishViewStepsRequest,
) (*pulumirpc.PublishViewStepsResponse, error) {
	info, ok := rs.tokens.Load(req.Token)
	if !ok {
		return nil, fmt.Errorf("token %s not found", req.Token)
	}
	viewOf := info.urn

	// Unmarshal the steps.
	steps, err := slice.MapError(req.Steps, func(step *pulumirpc.ViewStep) (Step, error) {
		return rs.unmarshalViewStep(viewOf, step, info.refresh)
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshaling steps: %w", err)
	}

	if len(steps) == 0 {
		return &pulumirpc.PublishViewStepsResponse{}, nil
	}

	// TODO validate steps like in the step generator?
	// e.g. sg.validateSteps(steps)

	events := rs.deployment.events

	// Raise the OnResourceStepPre event and keep track of the payload context.
	if events != nil {
		for _, step := range steps {
			payload, err := events.OnResourceStepPre(step)
			if err != nil {
				// TODO log
				return nil, fmt.Errorf("publishing view steps: %w", err)
			}

			info.mu.Lock()
			info.steps = append(info.steps, stepPre{
				step:    step,
				payload: payload,
			})
			info.mu.Unlock()
		}
	}

	return &pulumirpc.PublishViewStepsResponse{}, nil
}

func (rs *resourceStatusServer) unmarshalViewStep(
	viewOf resource.URN, step *pulumirpc.ViewStep, refresh bool,
) (Step, error) {
	status, err := rs.unmarshalStepStatus(step.GetStatus())
	if err != nil {
		return nil, err
	}

	op, err := rs.unmarshalStepOp(step.GetOp())
	if err != nil {
		return nil, err
	}

	old, err := rs.unmarshalViewStepState(viewOf, step.GetOld())
	if err != nil {
		return nil, err
	}
	if old != nil {
		// Lookup the actual old state.
		// TODO only do it for Update?
		urn := old.URN
		old = rs.getOldView(viewOf, urn)
		contract.Assertf(old != nil, "old state %s of view %s not found in previous deployment", urn, viewOf)
	}

	new, err := rs.unmarshalViewStepState(viewOf, step.GetNew())
	if err != nil {
		return nil, err
	}

	keys := slice.Prealloc[resource.PropertyKey](len(step.GetKeys()))
	for _, key := range step.GetKeys() {
		keys = append(keys, resource.PropertyKey(key))
	}

	diffs := slice.Prealloc[resource.PropertyKey](len(step.GetDiffs()))
	for _, diff := range step.GetDiffs() {
		diffs = append(diffs, resource.PropertyKey(diff))
	}

	detailedDiff := rs.unmarshalDetailedDiff(step)

	result := NewViewStep(rs.deployment, op, status, step.GetError(), old, new, keys, diffs, detailedDiff).(*ViewStep)
	if refresh {
		// If this is a refresh step, we need to set the result operation to the same as the original operation.
		result.resultOp = op
		result.op = OpRefresh
	}
	return result, nil
}

func (rs *resourceStatusServer) unmarshalDetailedDiff(step *pulumirpc.ViewStep) map[string]plugin.PropertyDiff {
	if !step.GetHasDetailedDiff() {
		return nil
	}

	detailedDiff := make(map[string]plugin.PropertyDiff)
	for k, v := range step.GetDetailedDiff() {
		var d plugin.DiffKind
		switch v.GetKind() {
		case pulumirpc.PropertyDiff_ADD:
			d = plugin.DiffAdd
		case pulumirpc.PropertyDiff_ADD_REPLACE:
			d = plugin.DiffAddReplace
		case pulumirpc.PropertyDiff_DELETE:
			d = plugin.DiffDelete
		case pulumirpc.PropertyDiff_DELETE_REPLACE:
			d = plugin.DiffDeleteReplace
		case pulumirpc.PropertyDiff_UPDATE:
			d = plugin.DiffUpdate
		case pulumirpc.PropertyDiff_UPDATE_REPLACE:
			d = plugin.DiffUpdateReplace
		default:
			// Consider unknown diff kinds to be simple updates.
			d = plugin.DiffUpdate
		}
		detailedDiff[k] = plugin.PropertyDiff{
			Kind:      d,
			InputDiff: v.GetInputDiff(),
		}
	}

	return detailedDiff
}

func (rs *resourceStatusServer) unmarshalViewStepState(
	viewOf resource.URN, state *pulumirpc.ViewStepState,
) (*resource.State, error) {
	if state == nil {
		return nil, nil
	}

	stateType := tokens.Type(state.GetType())
	stateURN := rs.deployment.generateURN(viewOf, stateType, state.GetName())

	// TODO parenting

	inputs, err := plugin.UnmarshalProperties(state.GetInputs(), plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshaling inputs: %w", err)
	}

	outputs, err := plugin.UnmarshalProperties(state.GetOutputs(), plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshaling outputs: %w", err)
	}

	return &resource.State{
		Custom:  false,
		ViewOf:  viewOf,
		Parent:  viewOf,
		URN:     stateURN,
		Type:    stateType,
		Inputs:  inputs,
		Outputs: outputs,
	}, nil
}

func (rs *resourceStatusServer) unmarshalStepOp(op pulumirpc.ViewStep_Op) (display.StepOp, error) {
	switch op {
	case pulumirpc.ViewStep_SAME:
		return OpSame, nil
	case pulumirpc.ViewStep_CREATE:
		return OpCreate, nil
	case pulumirpc.ViewStep_UPDATE:
		return OpUpdate, nil
	case pulumirpc.ViewStep_DELETE:
		return OpDelete, nil
	case pulumirpc.ViewStep_REPLACE:
		return OpReplace, nil
	case pulumirpc.ViewStep_CREATE_REPLACEMENT:
		return OpCreateReplacement, nil
	case pulumirpc.ViewStep_DELETE_REPLACED:
		return OpDeleteReplaced, nil
	case pulumirpc.ViewStep_READ:
		return OpRead, nil
	case pulumirpc.ViewStep_READ_REPLACEMENT:
		return OpReadReplacement, nil
	case pulumirpc.ViewStep_REFRESH:
		return OpRefresh, nil
	case pulumirpc.ViewStep_READ_DISCARD:
		return OpReadDiscard, nil
	case pulumirpc.ViewStep_DISCARD_REPLACED:
		return OpDiscardReplaced, nil
	case pulumirpc.ViewStep_REMOVE_PENDING_REPLACE:
		return OpRemovePendingReplace, nil
	case pulumirpc.ViewStep_IMPORT:
		return OpImport, nil
	case pulumirpc.ViewStep_IMPORT_REPLACEMENT:
		return OpImportReplacement, nil
	case pulumirpc.ViewStep_UNSPECIFIED:
		fallthrough
	default:
		return "", fmt.Errorf("unknown step op %v", op)
	}
}

func (rs *resourceStatusServer) unmarshalStepStatus(s pulumirpc.ViewStep_Status) (resource.Status, error) {
	switch s {
	case pulumirpc.ViewStep_OK:
		return resource.StatusOK, nil
	case pulumirpc.ViewStep_PARTIAL_FAILURE:
		return resource.StatusPartialFailure, nil
	case pulumirpc.ViewStep_UNKNOWN:
		return resource.StatusUnknown, nil
	default:
		return 0, fmt.Errorf("unknown step status %v", s)
	}
}

//nolint:unused
func (rs *resourceStatusServer) unmarshalPropertyDiffKind(kind pulumirpc.PropertyDiff_Kind) (plugin.DiffKind, error) {
	switch kind {
	case pulumirpc.PropertyDiff_ADD:
		return plugin.DiffAdd, nil
	case pulumirpc.PropertyDiff_ADD_REPLACE:
		return plugin.DiffAddReplace, nil
	case pulumirpc.PropertyDiff_DELETE:
		return plugin.DiffDelete, nil
	case pulumirpc.PropertyDiff_DELETE_REPLACE:
		return plugin.DiffDeleteReplace, nil
	case pulumirpc.PropertyDiff_UPDATE:
		return plugin.DiffUpdate, nil
	case pulumirpc.PropertyDiff_UPDATE_REPLACE:
		return plugin.DiffUpdateReplace, nil
	default:
		return 0, fmt.Errorf("unknown property diff kind %v", kind)
	}
}

// getViews returns the set of views for a given URN.
func (rs *resourceStatusServer) getOldView(viewOf resource.URN, urn resource.URN) *resource.State {
	for _, res := range rs.deployment.prev.Resources {
		if res.ViewOf == viewOf && res.URN == urn {
			return res
		}
	}
	return nil
}
