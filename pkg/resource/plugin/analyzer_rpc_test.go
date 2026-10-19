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

package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// remediatingAnalyzerClient records the properties of the last Remediate request, and returns a remediation with
// the given properties.
type remediatingAnalyzerClient struct {
	pulumirpc.AnalyzerClient
	sent       *structpb.Struct
	remediated *structpb.Struct
}

func (c *remediatingAnalyzerClient) Remediate(
	_ context.Context, req *pulumirpc.AnalyzeRequest, _ ...grpc.CallOption,
) (*pulumirpc.RemediateResponse, error) {
	c.sent = req.GetProperties()
	return &pulumirpc.RemediateResponse{Remediations: []*pulumirpc.Remediation{{Properties: c.remediated}}}, nil
}

// Policy packs only understand unknowns as sentinel strings, in what they're sent and what they return.
func TestAnalyzerUnknownsAsSentinels(t *testing.T) {
	t.Parallel()

	unknown := structpb.NewStringValue(sig.UnknownStringValue)
	secretUnknown := structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.Secret),
		"value": unknown,
	}})
	client := &remediatingAnalyzerClient{remediated: &structpb.Struct{Fields: map[string]*structpb.Value{
		"id":   unknown,
		"tags": structpb.NewListValue(&structpb.ListValue{Values: []*structpb.Value{secretUnknown}}),
		"name": structpb.NewStringValue("web"),
	}}}
	computed := property.New(property.Computed)

	resp, err := NewAnalyzerWithClient("pack", client).Remediate(t.Context(), AnalyzerResource{
		Type: "pkg:index:Res",
		Properties: property.NewMap(map[string]property.Value{
			"id":   computed,
			"tags": property.New([]property.Value{computed.WithSecret(true)}),
			"name": property.New("web"),
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, unknown.AsInterface(), client.sent.GetFields()["id"].AsInterface())
	assert.Equal(t, []any{secretUnknown.AsInterface()}, client.sent.GetFields()["tags"].AsInterface())
	assert.Equal(t, "web", client.sent.GetFields()["name"].GetStringValue())

	require.Len(t, resp.Remediations, 1)
	props := resp.Remediations[0].Properties
	assert.True(t, props.Get("id").IsComputed())
	tag := props.Get("tags").AsArray().Get(0)
	assert.True(t, tag.IsComputed())
	assert.True(t, tag.Secret())
	assert.Equal(t, "web", props.Get("name").AsString())
}
