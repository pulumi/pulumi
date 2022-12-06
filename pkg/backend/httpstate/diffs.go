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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
)

type deploymentDiffState struct {
	lastSavedDeployment json.RawMessage
	sequenceNumber      int
	minimalDiffSize     int
}

type deploymentDiff struct {
	sequenceNumber  int
	checkpointHash  string
	deploymentDelta json.RawMessage
}

func newDeploymentDiffState(minimalDiffSize int) *deploymentDiffState {
	return &deploymentDiffState{
		sequenceNumber:  1,
		minimalDiffSize: minimalDiffSize,
	}
}

func (dds *deploymentDiffState) SequenceNumber() int {
	return dds.sequenceNumber
}

func (dds *deploymentDiffState) CanDiff() bool {
	return dds.lastSavedDeployment != nil
}

// Size-based heuristics trying to estimate if the diff method will be
// worth it and take less time than sending the entire deployment.
func (dds *deploymentDiffState) ShouldDiff(new *apitype.DeploymentV3) bool {
	if !dds.CanDiff() {
		return false
	}
	if len(dds.lastSavedDeployment) < dds.minimalDiffSize {
		return false
	}
	w := &maxLengthWriter{maxLength: dds.minimalDiffSize}
	_, err := client.MarshalUntypedDeployment(new).WriteTo(w)
	if err != nil && err.Error() == "maxLength exceeded" {
		return true
	}
	return false
}

type maxLengthWriter struct {
	maxLength     int
	currentLength int
}

func (sw *maxLengthWriter) Write(data []byte) (int, error) {
	n := len(data)
	sw.currentLength += n
	if sw.currentLength > sw.maxLength {
		return 0, fmt.Errorf("maxLength exceeded")
	}
	return n, nil
}

func (dds *deploymentDiffState) Diff(ctx context.Context, deployment *apitype.DeploymentV3) (deploymentDiff, error) {

	if !dds.CanDiff() {
		return deploymentDiff{}, fmt.Errorf("Diff() cannot be called before Saved()")
	}

	tracingSpan, childCtx := opentracing.StartSpanFromContext(ctx, "Diff")
	defer tracingSpan.Finish()

	before := dds.lastSavedDeployment

	afterBuf := bytes.Buffer{}
	if _, err := client.MarshalUntypedDeployment(deployment).WriteTo(&afterBuf); err != nil {
		return deploymentDiff{}, fmt.Errorf("marshalUntypedDeployment failed: %v", err)
	}

	var checkpointHash string
	checkpointHashReady := &sync.WaitGroup{}

	checkpointHashReady.Add(1)
	go func() {
		defer checkpointHashReady.Done()
		checkpointHash = dds.computeHash(childCtx, afterBuf.Bytes())
	}()

	delta, err := dds.computeEdits(childCtx, string(before), afterBuf.String())
	if err != nil {
		return deploymentDiff{}, fmt.Errorf("Cannot marshal the edits: %v", err)
	}

	checkpointHashReady.Wait()

	tracingSpan.SetTag("before", len(before))
	tracingSpan.SetTag("after", afterBuf.Len())
	tracingSpan.SetTag("diff", len(delta))
	tracingSpan.SetTag("compression", 100.0*float64(len(delta))/float64(afterBuf.Len()))
	tracingSpan.SetTag("hash", checkpointHash)

	diff := deploymentDiff{
		checkpointHash:  checkpointHash,
		deploymentDelta: delta,
		sequenceNumber:  dds.sequenceNumber,
	}

	return diff, nil
}

// Indicates that a deployment was just saved to the service.
func (dds *deploymentDiffState) Saved(ctx context.Context, deployment *apitype.DeploymentV3) error {
	var buf bytes.Buffer
	_, err := client.MarshalUntypedDeployment(deployment).WriteTo(&buf)
	if err != nil {
		return err
	}
	dds.lastSavedDeployment = buf.Bytes()
	dds.sequenceNumber++
	return nil
}

func (*deploymentDiffState) computeHash(ctx context.Context, deployment json.RawMessage) string {
	tracingSpan, _ := opentracing.StartSpanFromContext(ctx, "computeHash")
	defer tracingSpan.Finish()
	hash := sha256.Sum256(deployment)
	return hex.EncodeToString(hash[:])
}

func (*deploymentDiffState) computeEdits(ctx context.Context, before, after string) (json.RawMessage, error) {
	tracingSpan, _ := opentracing.StartSpanFromContext(ctx, "computeEdits")
	defer tracingSpan.Finish()

	edits := myers.ComputeEdits(span.URIFromURI(""), before, after)

	delta, err := json.Marshal(edits)
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal the edits: %v", err)
	}

	return delta, nil
}
