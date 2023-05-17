//go:build !go1.20

// Copyright 2016-2022, Pulumi Corporation.
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	opentracing "github.com/opentracing/opentracing-go"
)

type deployment struct {
	raw json.RawMessage
	buf *bytes.Buffer
}

func (dds *deploymentDiffState) MarshalDeployment(d *apitype.DeploymentV3) (deployment, error) {
	var b *bytes.Buffer
	if dds.buffer != nil {
		b, dds.buffer = dds.buffer, nil
	} else {
		b = &bytes.Buffer{}
	}

	if err := client.MarshalUntypedDeployment(b, d); err != nil {
		return deployment{}, err
	}
	return deployment{raw: json.RawMessage(b.Bytes()), buf: b}, nil
}

func (*deploymentDiffState) computeEdits(ctx context.Context, before, after deployment) (json.RawMessage, error) {
	tracingSpan, _ := opentracing.StartSpanFromContext(ctx, "computeEdits")
	defer tracingSpan.Finish()

	edits := myers.ComputeEdits(span.URIFromURI(""), string(before.raw), string(after.raw))

	delta, err := json.Marshal(edits)
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal the edits: %v", err)
	}

	return delta, nil
}
