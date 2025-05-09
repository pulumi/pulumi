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

package deploytest

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ResourceStatus struct {
	conn   *grpc.ClientConn
	client pulumirpc.ResourceStatusClient
}

func NewResourceStatus(address string) (*ResourceStatus, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to the resource status service: %w", err)
	}
	client := pulumirpc.NewResourceStatusClient(conn)
	return &ResourceStatus{
		conn:   conn,
		client: client,
	}, nil
}

func (rs *ResourceStatus) Close() error {
	return rs.conn.Close()
}

func (rs *ResourceStatus) PublishViewSteps(token string, steps []ViewStep) error {
	marshaledSteps, err := slice.MapError(steps, rs.marshalStep)
	if err != nil {
		return fmt.Errorf("marshaling steps: %w", err)
	}
	req := &pulumirpc.PublishViewStepsRequest{
		Token: token,
		Steps: marshaledSteps,
	}
	_, err = rs.client.PublishViewSteps(context.Background(), req)
	if err != nil {
		return fmt.Errorf("publishing view steps: %w", err)
	}
	return nil
}

func (rs *ResourceStatus) marshalStep(step ViewStep) (*pulumirpc.ViewStep, error) {
	return &pulumirpc.ViewStep{
		Op:     rs.marshalOp(step.Op),
		Status: rs.marshalStatus(step.Status),
		Error:  step.Error,
		Old:    rs.marshalState(step.Old),
		New:    rs.marshalState(step.New),
	}, nil
}

func (rs *ResourceStatus) marshalOp(op display.StepOp) pulumirpc.ViewStep_Op {
	switch op {
	case deploy.OpSame:
		return pulumirpc.ViewStep_SAME
	case deploy.OpCreate:
		return pulumirpc.ViewStep_CREATE
	case deploy.OpUpdate:
		return pulumirpc.ViewStep_UPDATE
	case deploy.OpDelete:
		return pulumirpc.ViewStep_DELETE
	case deploy.OpReplace:
		return pulumirpc.ViewStep_REPLACE
	case deploy.OpCreateReplacement:
		return pulumirpc.ViewStep_CREATE_REPLACEMENT
	case deploy.OpDeleteReplaced:
		return pulumirpc.ViewStep_DELETE_REPLACED
	case deploy.OpRead:
		return pulumirpc.ViewStep_READ
	case deploy.OpReadReplacement:
		return pulumirpc.ViewStep_READ_REPLACEMENT
	case deploy.OpRefresh:
		return pulumirpc.ViewStep_REFRESH
	case deploy.OpReadDiscard:
		return pulumirpc.ViewStep_READ_DISCARD
	case deploy.OpDiscardReplaced:
		return pulumirpc.ViewStep_DISCARD_REPLACED
	case deploy.OpRemovePendingReplace:
		return pulumirpc.ViewStep_REMOVE_PENDING_REPLACE
	case deploy.OpImport:
		return pulumirpc.ViewStep_IMPORT
	case deploy.OpImportReplacement:
		return pulumirpc.ViewStep_IMPORT_REPLACEMENT
	default:
		panic(fmt.Errorf("unknown step op %v", op))
	}
}

func (rs *ResourceStatus) marshalStatus(status resource.Status) pulumirpc.ViewStep_Status {
	switch status {
	case resource.StatusOK:
		return pulumirpc.ViewStep_OK
	case resource.StatusPartialFailure:
		return pulumirpc.ViewStep_PARTIAL_FAILURE
	case resource.StatusUnknown:
		return pulumirpc.ViewStep_UNKNOWN
	default:
		panic(fmt.Errorf("unknown status %v", status))
	}
}

func (rs *ResourceStatus) marshalState(state *ViewStepState) *pulumirpc.ViewStepState {
	if state == nil {
		return nil
	}

	inputs, err := plugin.MarshalProperties(state.Inputs, plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		panic(fmt.Errorf("marshaling inputs: %w", err))
	}

	outputs, err := plugin.MarshalProperties(state.Outputs, plugin.MarshalOptions{
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		panic(fmt.Errorf("marshaling outputs: %w", err))
	}

	return &pulumirpc.ViewStepState{
		Type:       string(state.Type),
		Name:       state.Name,
		ParentType: string(state.ParentType),
		ParentName: state.ParentName,
		Inputs:     inputs,
		Outputs:    outputs,
	}
}

type ViewStep struct {
	Op     display.StepOp
	Status resource.Status
	Error  string
	Old    *ViewStepState
	New    *ViewStepState
	// TODO keys, diffs, detailedDiff
}

type ViewStepState struct {
	Type       tokens.Type
	Name       string
	ParentType tokens.Type
	ParentName string
	Inputs     resource.PropertyMap
	Outputs    resource.PropertyMap
}
