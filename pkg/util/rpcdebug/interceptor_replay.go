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

package rpcdebug

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReplayInterceptorOptions struct {
	LogFile string
	Mutex   *sync.Mutex
}

type ReplayInterceptor struct {
	entries []debugInterceptorLogEntry
	mutex   *sync.Mutex
}

func NewReplayInterceptor(opts ReplayInterceptorOptions) (*ReplayInterceptor, error) {
	if opts.Mutex == nil {
		return nil, fmt.Errorf("Mutex option is required")
	}
	if opts.LogFile == "" {
		return nil, fmt.Errorf("LogFile option is required")
	}
	f, err := os.Open(opts.LogFile)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(f)
	entries := []debugInterceptorLogEntry{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry debugInterceptorLogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &ReplayInterceptor{
		entries: entries,
		mutex:   opts.Mutex,
	}, nil
}

func (i *ReplayInterceptor) ClientInterceptor(opts LogOptions) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, gopts ...grpc.CallOption) error {

		if strings.HasPrefix(method, "/pulumirpc.ResourceProvider") {
			reqJ, err := transcode(req)
			if err != nil {
				panic(fmt.Errorf("ReplayInterceptor failed to transcode: %w", err))
			}

			found, _, entry := i.popEntry(method, reqJ, opts.Metadata)
			if !found {
				return status.Errorf(codes.FailedPrecondition,
					"Cannot find matching logs of a call with method=%q reqJ=%s metadata=%v",
					method, reqJ, opts.Metadata)
			}

			if i.isUnimplemented(entry) {
				return status.Errorf(codes.Unimplemented, "%s Unimplemented", method)
			}

			if err := transcodeBack(entry.Response, reply); err != nil {
				return status.Errorf(codes.FailedPrecondition,
					"ReplayInterceptor failed to transcodeBack from %q (%d bytes): %w",
					entry.Response, len(entry.Response), err)
			}

			return nil
		}

		return invoker(ctx, method, req, reply, cc, gopts...)
	}
}

func (*ReplayInterceptor) isUnimplemented(entry debugInterceptorLogEntry) bool {
	for _, e := range entry.Errors {
		if strings.Contains(strings.ToLower(e), "unimplemented") {
			return true
		}
	}
	return false
}

func (i *ReplayInterceptor) popEntry(method string, req json.RawMessage, meta interface{}) (bool, int, debugInterceptorLogEntry) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	ok, index, e := i.findEntry(method, req, meta)
	if ok {
		i.removeEntry(index)
	}
	return ok, index, e
}

func (i *ReplayInterceptor) removeEntry(index int) {
	i.entries = append(i.entries[0:index], i.entries[index+1:]...)
}

func (i *ReplayInterceptor) findEntry(method string, req json.RawMessage, metadata interface{}) (bool, int, debugInterceptorLogEntry) {
	ref := debugInterceptorLogEntry{Method: method, Request: req, Metadata: metadata}
	for k, e := range i.entries {
		if i.entriesMatch(e, ref) {
			return true, k, e
		}
	}
	return false, 0, debugInterceptorLogEntry{}
}

func (*ReplayInterceptor) entriesMatch(a, b debugInterceptorLogEntry) bool {
	if a.Method != b.Method {
		return false
	}
	if !bytes.Equal(a.Request, b.Request) {
		return false
	}
	if !reflect.DeepEqual(a.Metadata, b.Metadata) {
		return false
	}
	return true
}

func (i *ReplayInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, gopts ...grpc.CallOption) (grpc.ClientStream, error) {
		panic("ReplayInterceptor caught client stream call")
	}
}

func (i *ReplayInterceptor) DialOptions(opts LogOptions) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(i.ClientInterceptor(opts)),
		grpc.WithChainStreamInterceptor(i.StreamClientInterceptor()),
	}
}
