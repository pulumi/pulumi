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
	"compress/gzip"
	"fmt"
	"io"

	"github.com/json-iterator/go"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

var jsonIterConfig = jsoniter.Config{SortMapKeys: true}.Froze()

// Implements io.WriterTo that writes canonical JSON in the apitype.UntypedDeployment format.
//
// Optimized for large checkpoints. Streams the data instead of buffering everything upfront.
//
// Injects newlines to allow efficient textual diffs over the JSON.
func MarshalUntypedDeployment(deployment *apitype.DeploymentV3) io.WriterTo {
	return &marshalUntypedDeployment{deployment}
}

type marshalUntypedDeployment struct {
	deployment *apitype.DeploymentV3
}

var _ io.WriterTo = &marshalUntypedDeployment{}

func (c *marshalUntypedDeployment) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}
	cfg := jsonIterConfig
	stream := cfg.BorrowStream(cw)
	defer cfg.ReturnStream(stream)
	err := c.writeToStream(stream)
	return cw.written, err
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
	if len(deployment.Resources) > 0 {
		stream.WriteMore()
		stream.WriteObjectField("resources")
		stream.WriteRaw("[\n")
		for i, r := range deployment.Resources {
			if i > 0 {
				stream.WriteRaw(",\n")
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
		stream.WriteRaw("[\n")
		for i, op := range deployment.PendingOperations {
			if i > 0 {
				stream.WriteRaw(",\n")
			}
			stream.WriteVal(op)
			if err = stream.Flush(); err != nil {
				return err
			}
		}
		stream.WriteRaw("\n]")
	}
	stream.WriteObjectEnd()
	return stream.Flush()
}

// Similar to marshalUntypedDeployment but in apitype.PatchUpdateVerbatimCheckpointRequest format.
type marshalPatchUpdateVerbatimCheckpointRequest struct {
	deployment     *apitype.DeploymentV3
	sequenceNumber int
}

var _ io.WriterTo = (*marshalPatchUpdateVerbatimCheckpointRequest)(nil)

func (c *marshalPatchUpdateVerbatimCheckpointRequest) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}
	cfg := jsonIterConfig
	stream := cfg.BorrowStream(cw)
	defer cfg.ReturnStream(stream)

	stream.WriteObjectStart()
	stream.WriteObjectField("version")
	stream.WriteInt(3)
	stream.WriteMore()
	stream.WriteObjectField("untypedDeployment")

	md := &marshalUntypedDeployment{c.deployment}
	if err := md.writeToStream(stream); err != nil {
		return cw.written, err
	}

	stream.WriteMore()
	stream.WriteObjectField("sequenceNumber")
	stream.WriteInt(c.sequenceNumber)
	stream.WriteObjectEnd()
	err := stream.Flush()
	return cw.written, err
}

// Similar to marshalUntypedDeployment but in apitype.PatchUpdateCheckpointRequest format.
type marshalPatchUpdateCheckpointRequest struct {
	deployment *apitype.DeploymentV3
}

var _ io.WriterTo = (*marshalPatchUpdateVerbatimCheckpointRequest)(nil)

func (c *marshalPatchUpdateCheckpointRequest) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}
	cfg := jsonIterConfig
	stream := cfg.BorrowStream(cw)
	defer cfg.ReturnStream(stream)
	stream.WriteObjectStart()
	stream.WriteObjectField("isInvalid")
	stream.WriteBool(false)
	stream.WriteMore()
	stream.WriteObjectField("version")
	stream.WriteInt(3)
	if c.deployment != nil {
		stream.WriteMore()
		stream.WriteObjectField("deployment")
		md := &marshalUntypedDeployment{c.deployment}
		if err := md.writeToStream(stream); err != nil {
			return cw.written, err
		}
	}
	stream.WriteObjectEnd()
	err := stream.Flush()
	return cw.written, err
}

type countingWriter struct {
	w       io.Writer
	written int64
}

var _ io.Writer = (*countingWriter)(nil)

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if err == nil {
		cw.written += int64(n)
	}
	return n, err
}

type gzipEncodingWriterTo struct {
	innner io.WriterTo
}

var _ io.WriterTo = (*gzipEncodingWriterTo)(nil)

func (data *gzipEncodingWriterTo) WriteTo(w io.Writer) (int64, error) {
	logging.V(apiRequestDetailLogLevel).Infoln("compressing payload using gzip")
	compressedCounter := &countingWriter{w: w}
	gw := gzip.NewWriter(compressedCounter)
	uncompressedCounter := &countingWriter{w: gw}
	_, err := data.innner.WriteTo(uncompressedCounter)
	if err != nil {
		return 0, err
	}
	if err := gw.Close(); err != nil {
		return 0, err
	}
	logging.V(apiRequestDetailLogLevel).Infof("gzip compression ratio: %f, original size: %d bytes",
		float64(uncompressedCounter.written)/float64(compressedCounter.written), uncompressedCounter.written)
	return compressedCounter.written, nil
}

type jsonMarshalWriterTo struct {
	message interface{}
}

var _ io.WriterTo = (*jsonMarshalWriterTo)(nil)

func (m *jsonMarshalWriterTo) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}
	encoder := jsonIterConfig.NewEncoder(cw)
	if err := encoder.Encode(m.message); err != nil {
		return cw.written, fmt.Errorf("JSON marshalling error: %w", err)
	}
	return cw.written, nil
}

type bytesWriterTo struct {
	message []byte
}

var _ io.WriterTo = &bytesWriterTo{}

func (x *bytesWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(x.message)
	return int64(n), err
}

type limitWriter struct {
	buf      bytes.Buffer
	maxBytes int
	written  int64
}

var _ io.Writer = &limitWriter{}

func (lw *limitWriter) Overflow() bool {
	return lw.buf.Len() > lw.maxBytes
}

func (lw *limitWriter) Write(bytes []byte) (int, error) {
	if lw.Overflow() {
		n := len(bytes)
		lw.written += int64(n)
		return n, nil
	}
	n, err := lw.buf.Write(bytes)
	lw.written += int64(n)
	return n, err
}
