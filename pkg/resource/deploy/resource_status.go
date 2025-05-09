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
	"fmt"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

type resourceStatusServer struct {
	pulumirpc.UnsafeResourceStatusServer

	// The deployment to which this step generator belongs.
	deployment *Deployment

	// A channel to post steps back to the step generator.
	events chan<- SourceEvent

	// The address the server is listening on.
	address string

	// A map of tokens to URNs.
	tokens gsync.Map[string, *tokenInfo] // token -> tokenInfo
}

type tokenInfo struct {
	urn     resource.URN
	mu      sync.Mutex
	batches [][]Step
}

func newResourceStatusServer(deployment *Deployment, events chan<- SourceEvent) (*resourceStatusServer, error) {
	rs := &resourceStatusServer{
		deployment: deployment,
		events:     events,
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

func (rs *resourceStatusServer) ReserveToken(urn resource.URN) (string, error) {
	token, err := uuid.NewV4()
	if err != nil {
		return "", fmt.Errorf("creating token: %w", err)
	}
	tokenString := token.String()
	rs.tokens.Store(tokenString, &tokenInfo{urn: urn})
	return tokenString, nil
}

func (rs *resourceStatusServer) ReleaseToken(token string) {
	if token == "" {
		return
	}

	info, ok := rs.tokens.LoadAndDelete(token)
	if !ok {
		return
	}

	info.mu.Lock()
	batches := info.batches
	info.mu.Unlock()

	for _, steps := range batches {
		// Publish the steps in the batch.
		rs.events <- &additionalStepsEvent{
			steps: steps,
		}
	}
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
		return rs.unmarshalViewStep(viewOf, step)
	})
	if err != nil {
		return nil, fmt.Errorf("unmarshaling steps: %w", err)
	}

	// Save the steps.
	if len(steps) > 0 {
		info.mu.Lock()
		defer info.mu.Unlock()
		info.batches = append(info.batches, steps)
	}

	return &pulumirpc.PublishViewStepsResponse{}, nil
}

func (rs *resourceStatusServer) unmarshalViewStep(viewOf resource.URN, step *pulumirpc.ViewStep) (Step, error) {
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

	new, err := rs.unmarshalViewStepState(viewOf, step.GetNew())
	if err != nil {
		return nil, err
	}

	// TODO keys, diffs, detailedDiffs

	return NewViewStep(rs.deployment, op, status, step.GetError(), old, new, nil, nil, nil), nil
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

	outputs, err := plugin.UnmarshalProperties(state.GetInputs(), plugin.MarshalOptions{
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
