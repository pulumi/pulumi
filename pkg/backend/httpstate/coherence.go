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

package httpstate

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

type stackOutputsRecorder struct {
	outputs resource.PropertyMap
	seen    bool
	deleted bool
}

func (r *stackOutputsRecorder) record(e engine.Event) {
	if r == nil || r.deleted || e.Type != engine.ResourceOutputsEvent {
		return
	}
	payload, ok := e.Payload().(engine.ResourceOutputsEventPayload)
	if !ok || payload.Metadata.URN.QualifiedType() != resource.RootStackType {
		return
	}
	if payload.Metadata.Op == deploy.OpDelete {
		r.outputs, r.seen, r.deleted = resource.PropertyMap{}, true, true
		return
	}
	if state := payload.Metadata.New; state != nil && state.State != nil {
		r.outputs, r.seen = state.State.Outputs, true
	}
}

func (r *stackOutputsRecorder) response(
	ctx context.Context, sm secrets.Manager,
) (*apitype.StackOutputsResponse, error) {
	if r == nil || sm == nil || !r.seen {
		return nil, nil
	}

	serialized, err := stack.SerializeProperties(ctx, r.outputs, sm.Encrypter(), false /* showSecrets */)
	if err != nil {
		return nil, fmt.Errorf("serializing stack outputs: %w", err)
	}
	return &apitype.StackOutputsResponse{
		Outputs: serialized,
		SecretsProviders: &apitype.SecretsProvidersV1{
			Type:  sm.Type(),
			State: sm.State(),
		},
	}, nil
}
