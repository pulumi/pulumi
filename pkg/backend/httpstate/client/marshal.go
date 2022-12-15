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

package client

import (
	"bytes"
	"encoding/json"
	"io"
	"math"

	"github.com/json-iterator/go"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var jsonIterConfig = jsoniter.Config{SortMapKeys: true}.Froze()

// Marshals to canonical JSON in the apitype.UntypedDeployment format.
//
// Optimized for large checkpoints.
//
// Injects newlines to allow efficient textual diffs over the JSON. Textual diffs currently use O(N^2) memory in the
// number of newlines, so the injection needs to be conservative. Currently it limits to up to 1024 newlines which would
// result in max 8MB memory use by the algorithm.
func MarshalUntypedDeployment(deployment *apitype.DeploymentV3) (json.RawMessage, error) {
	var buf bytes.Buffer
	var md = &marshalUntypedDeployment{deployment}
	if err := md.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshalVerbatimCheckpointRequest(req apitype.PatchUpdateVerbatimCheckpointRequest) (json.RawMessage, error) {
	// Unlike encoding/json, using jsonIter here will not reindent req.UntypedDeployment, which is what is needed
	// for the Verbatim protocol.
	return jsonIterConfig.Marshal(req)
}

type marshalUntypedDeployment struct {
	deployment *apitype.DeploymentV3
}

func (c *marshalUntypedDeployment) Write(w io.Writer) error {
	cfg := jsonIterConfig
	stream := cfg.BorrowStream(w)
	defer cfg.ReturnStream(stream)
	err := c.writeToStream(stream)
	return err
}

func (c *marshalUntypedDeployment) writeToStream(stream *jsoniter.Stream) error {
	stream.WriteObjectStart()
	stream.WriteObjectField("version")
	stream.WriteInt(3)
	stream.WriteMore()
	stream.WriteObjectField("deployment")
	c.writeDeploymentV3(stream)
	stream.WriteObjectEnd()
	return stream.Flush()
}

func (c *marshalUntypedDeployment) writeDeploymentV3(stream *jsoniter.Stream) (err error) {
	deployment := c.deployment
	stream.WriteObjectStart()
	stream.WriteObjectField("manifest")
	stream.WriteVal(deployment.Manifest)
	if deployment.SecretsProviders != nil {
		stream.WriteMore()
		stream.WriteObjectField("secrets_providers")
		stream.WriteVal(deployment.SecretsProviders)
	}
	if err = stream.Flush(); err != nil {
		return err
	}
	nResources := len(deployment.Resources)

	maxNewlines := 1024 - 2
	newlinePeriod := int(math.Ceil(float64(nResources) / float64(maxNewlines)))

	if nResources > 0 {
		stream.WriteMore()
		stream.WriteObjectField("resources")
		stream.WriteRaw("[\n")
		for i, r := range deployment.Resources {
			if i > 0 {
				stream.WriteRaw(",")
				if (nResources <= maxNewlines) || (i%newlinePeriod == 0) {
					stream.WriteRaw("\n")
				}
			}
			stream.WriteVal(r)
			if err = stream.Flush(); err != nil {
				return err
			}
		}
		stream.WriteRaw("\n]")
	}
	if len(deployment.PendingOperations) > 0 {
		stream.WriteMore()
		stream.WriteObjectField("pendingOperations")
		stream.WriteVal(deployment.PendingOperations)
	}
	stream.WriteObjectEnd()
	return stream.Flush()
}
