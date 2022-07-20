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

package rpcutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"google.golang.org/grpc"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Logs all gRPC converations in JSON format.
//
// To enable, PULUMI_DEBUG_GRPC must be set, pointing to a file to
// append logs to.
//
// For example:
//
//     PULUMI_DEBUG_GPRC=grpc.json pulumi preview
//     jq . grpc.json
//
func debugServerInterceptor() grpc.UnaryServerInterceptor {
	return debugInterceptorInstance
}

type debugInterceptorLogEntry struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
	Errors   []string        `json:"errors,omitempty"`
}

type debugInterceptor struct {
	logFile string
	mutex   sync.Mutex
}

func newDebugInterceptor() *debugInterceptor {
	logFile := os.Getenv("PULUMI_DEBUG_GRPC")
	i := &debugInterceptor{logFile: logFile}
	if logFile != "" {
		i.clearLogFile()
	}
	return i
}

func (i *debugInterceptor) interceptor() grpc.UnaryServerInterceptor {
	if i.logFile == "" {
		return i.noInterceptor()
	}
	return i.loggingInterceptor()
}

func (i *debugInterceptor) loggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var err error
		log := debugInterceptorLogEntry{
			Method: info.FullMethod,
		}
		if log.Request, err = i.transcode(req); err != nil {
			i.track(&log, err)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			i.track(&log, err)
		}
		if log.Response, err = i.transcode(resp); err != nil {
			i.track(&log, err)
		}
		i.record(log)
		return resp, err
	}
}

func (i *debugInterceptor) clearLogFile() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	err := ioutil.WriteFile(i.logFile, []byte{}, 0644)
	contract.AssertNoError(err)
}

func (i *debugInterceptor) record(log debugInterceptorLogEntry) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	f, err := os.OpenFile(i.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	contract.AssertNoError(err)
	defer contract.IgnoreClose(f)

	err = json.NewEncoder(f).Encode(log)
	contract.AssertNoError(err)
}

func (i *debugInterceptor) noInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{},
		_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}

func (*debugInterceptor) track(log *debugInterceptorLogEntry, err error) {
	log.Errors = append(log.Errors, err.Error())
}

func (*debugInterceptor) transcode(obj interface{}) (json.RawMessage, error) {
	m, ok := obj.(proto.Message)
	if !ok {
		return json.RawMessage("null"),
			fmt.Errorf("Failed to decode, expecting proto.Message: %v", m)
	}

	jsonSer := jsonpb.Marshaler{}
	buf := bytes.Buffer{}
	if err := jsonSer.Marshal(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var debugInterceptorInstance grpc.UnaryServerInterceptor = newDebugInterceptor().interceptor()
