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
	"io"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	interceptors "github.com/pulumi/pulumi/pkg/v3/util/rpcdebug"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
)

// resourceStatusServer implements the gRPC server for the resource status service.
type resourceStatusServer struct {
	io.Closer

	pulumirpc.UnsafeResourceStatusServer

	cancel chan bool
	handle rpcutil.ServeHandle

	// The deployment to which this step generator belongs.
	deployment *Deployment

	// The address the server is listening on.
	address string

	// A map of URNs to tokens.
	urns gsync.Map[resource.URN, string] // urn -> token

	// A map of tokens to URNs.
	tokens gsync.Map[string, *tokenInfo] // token -> tokenInfo

	// Protects refresh steps
	refreshStepsLock sync.Mutex

	// Refresh steps that were published to the status server.
	refreshSteps []Step
}

// tokenInfo holds information about a token reserved for a resource status operation.
type tokenInfo struct {
	// The owning resource URN for this token.
	urn resource.URN

	// Whether this token is for a refresh operation.
	refresh bool

	// If this is a refresh operation, whether the steps should be persisted.
	persisted bool

	// Protects the steps slice.
	mu sync.Mutex

	// Steps that were published with this token.
	steps []stepInfo
}

// stepInfo holds the step and the payload context associated with the OnResourceStepPre event.
type stepInfo struct {
	// The step.
	step Step

	// The payload context associated with the OnResourceStepPre event.
	payload any
}

// newResourceStatusServer creates a new resource status server and starts listening for incoming requests.
func newResourceStatusServer(deployment *Deployment) (*resourceStatusServer, error) {
	cancel := make(chan bool)

	rs := &resourceStatusServer{
		deployment: deployment,
		urns:       gsync.Map[resource.URN, string]{},
		tokens:     gsync.Map[string, *tokenInfo]{},
	}

	// Fire up a gRPC server and start listening for incomings.
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceStatusServer(srv, rs)
			return nil
		},
		Options: resourceStatusServeOptions(deployment.ctx, env.DebugGRPC.Value()),
	})
	if err != nil {
		return nil, err
	}

	rs.address = fmt.Sprintf("127.0.0.1:%d", handle.Port)
	rs.cancel = cancel
	rs.handle = handle

	return rs, nil
}

// resourceStatusServeOptions returns the gRPC server options for the resource status server.
func resourceStatusServeOptions(ctx *plugin.Context, logFile string) []grpc.ServerOption {
	var serveOpts []grpc.ServerOption
	if logFile != "" {
		di, err := interceptors.NewDebugInterceptor(interceptors.DebugInterceptorOptions{
			LogFile: logFile,
			Mutex:   ctx.DebugTraceMutex,
		})
		if err != nil {
			// ignoring
			return nil
		}
		metadata := map[string]any{
			"mode": "server",
		}
		serveOpts = append(serveOpts, di.ServerOptions(interceptors.LogOptions{
			Metadata: metadata,
		})...)
	}
	return serveOpts
}

// Close stops the resource status server.
func (rs *resourceStatusServer) Close() error {
	if rs != nil && rs.cancel != nil {
		rs.cancel <- true
		err := <-rs.handle.Done
		rs.cancel = nil
		return err
	}
	return nil
}

// Address returns the address the resource status server is listening on.
func (rs *resourceStatusServer) Address() string {
	if rs == nil {
		return ""
	}

	return rs.address
}

// ReserveToken reserves a token for a resource status operation.
func (rs *resourceStatusServer) ReserveToken(urn resource.URN, refresh, persisted bool) (string, error) {
	if rs == nil {
		return "", nil
	}
	token, _, err := rs.reserveToken(urn, refresh, persisted)
	return token, err
}

// reserveToken reserves a token for a resource status operation, and returns the token and tokenInfo.
func (rs *resourceStatusServer) reserveToken(urn resource.URN, refresh, persisted bool) (string, *tokenInfo, error) {
	if rs == nil {
		return "", nil, nil
	}

	logging.V(5).Infof("Reserving token for %s (refresh: %t)", urn, refresh)

	token, err := uuid.NewV4()
	if err != nil {
		return "", nil, fmt.Errorf("creating token: %w", err)
	}
	tokenString := token.String()
	rs.urns.Store(urn, tokenString)
	info := &tokenInfo{
		urn:       urn,
		refresh:   refresh,
		persisted: persisted,
	}
	rs.tokens.Store(tokenString, info)
	return tokenString, info, nil
}

// ReleaseToken returns the view steps published for the URN and releases the associated token so that no further
// steps can be published for it.
func (rs *resourceStatusServer) ReleaseToken(urn resource.URN) []stepInfo {
	if rs == nil {
		return nil
	}

	logging.V(5).Infof("ReleaseToken %s", urn)

	if urn == "" {
		return nil
	}

	token, ok := rs.urns.LoadAndDelete(urn)
	if !ok {
		return nil
	}

	info, ok := rs.tokens.LoadAndDelete(token)
	if !ok {
		return nil
	}

	info.mu.Lock()
	defer info.mu.Unlock()
	return info.steps
}

// Returns refresh steps that were published to the status server.
func (rs *resourceStatusServer) RefreshSteps() map[resource.URN]Step {
	if rs == nil {
		return nil
	}

	rs.refreshStepsLock.Lock()
	steps := rs.refreshSteps
	rs.refreshStepsLock.Unlock()

	if len(steps) == 0 {
		return nil
	}

	result := make(map[resource.URN]Step, len(steps))
	for _, step := range steps {
		result[step.URN()] = step
	}
	return result
}

// PublishViewSteps publishes view steps for a resource status operation.
func (rs *resourceStatusServer) PublishViewSteps(ctx context.Context,
	req *pulumirpc.PublishViewStepsRequest,
) (*pulumirpc.PublishViewStepsResponse, error) {
	logging.V(5).Infof("ResourceStatus.PublishViewSteps received for token %s", req.Token)

	info, ok := rs.tokens.Load(req.Token)
	if !ok {
		logging.V(5).Infof("ResourceStatus: token %s not found", req.Token)
		return nil, fmt.Errorf("token %s not found", req.Token)
	}
	viewOf := info.urn

	if len(req.Steps) == 0 {
		return &pulumirpc.PublishViewStepsResponse{}, nil
	}

	// Unmarshal the steps.
	steps, err := slice.MapError(req.Steps, func(step *pulumirpc.ViewStep) (Step, error) {
		return rs.unmarshalViewStep(viewOf, step, info.refresh, info.persisted)
	})
	if err != nil {
		logging.V(5).Infof("ResourceStatus: error unmarshaling steps: %v", err)
		return nil, fmt.Errorf("unmarshaling steps: %w", err)
	}

	// Save any refresh steps.
	for _, step := range steps {
		if step.Op() == OpRefresh {
			rs.refreshStepsLock.Lock()
			rs.refreshSteps = append(rs.refreshSteps, step)
			rs.refreshStepsLock.Unlock()
		}
	}

	// Publish the steps.
	if err := rs.publishViewSteps(info, steps); err != nil {
		return nil, err
	}

	return &pulumirpc.PublishViewStepsResponse{}, nil
}

// publishViewStepsWithTokenInfo publishes the view steps.
func (rs *resourceStatusServer) publishViewSteps(info *tokenInfo, steps []Step) error {
	if rs == nil {
		return nil
	}

	contract.Requiref(info != nil, "info", "must not be nil")

	events := rs.deployment.events
	if events == nil {
		return nil
	}

	// Raise the OnResourceStepPre event and keep track of the payload context.
	for _, step := range steps {
		payload, err := events.OnResourceStepPre(step)
		if err != nil {
			logging.V(5).Infof("ResourceStatus: error publishing view steps: %v", err)
			return fmt.Errorf("publishing view steps: %w", err)
		}

		info.mu.Lock()
		info.steps = append(info.steps, stepInfo{
			step:    step,
			payload: payload,
		})
		info.mu.Unlock()
	}

	return nil
}

func (rs *resourceStatusServer) unmarshalViewStep(
	viewOf resource.URN, step *pulumirpc.ViewStep, refresh bool, persisted bool,
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
		publishedOld := old

		// Look up the actual old resource state from the previous deployment.
		// This needs to be the actual pointer to the old resource state for the snapshot to be updated correctly.
		old = rs.deployment.olds[publishedOld.URN]
		contract.Assertf(old != nil, "old resource state %s not found", publishedOld.URN)
		contract.Assertf(old.ViewOf == viewOf, "old resource state should be a view of %s", viewOf)

		// Update its inputs and outputs to match what was published.
		old.Inputs = publishedOld.Inputs
		old.Outputs = publishedOld.Outputs
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

	var resultOp display.StepOp
	if refresh {
		// If this is a refresh step, we need to set the result operation to the same as the original operation.
		resultOp = op
		op = OpRefresh
	}

	return NewViewStep(
		rs.deployment, op, status, step.GetError(),
		old, new, keys, diffs, detailedDiff, resultOp, persisted), nil
}

// unmarshalDetailedDiff unmarshals the detailed diff from a ViewStep.
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

// unmarshalViewStepState unmarshals a ViewStepState into a resource.State.
func (rs *resourceStatusServer) unmarshalViewStepState(
	viewOf resource.URN, state *pulumirpc.ViewStepState,
) (*resource.State, error) {
	if state == nil {
		return nil, nil
	}

	stateType := tokens.Type(state.GetType())
	stateURN := rs.deployment.generateURN(viewOf, stateType, state.GetName())

	// TODO[pulumi/pulumi#19704]: Implement parenting.

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

// unmarshalStepOp unmarshals a pulumirpc.ViewStep_Op into a display.StepOp.
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

// unmarshalStepStatus unmarshals a pulumirpc.ViewStep_Status into a resource.Status.
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
