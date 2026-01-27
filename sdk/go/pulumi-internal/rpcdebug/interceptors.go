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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"google.golang.org/grpc"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
)

type DebugInterceptor struct {
	logFile string
	mutex   *sync.Mutex
}

type DebugInterceptorOptions struct {
	LogFile string
	Mutex   *sync.Mutex
}

type LogOptions struct {
	Metadata any
}

var interceptors gsync.Map[string, *DebugInterceptor]

func NewDebugInterceptor(opts DebugInterceptorOptions) (*DebugInterceptor, error) {
	if opts.LogFile == "" {
		return nil, errors.New("logFile cannot be empty")
	}

	// Reuse an instance if found in the global cache.
	if interceptor, ok := interceptors.Load(opts.LogFile); ok {
		return interceptor, nil
	}

	i := &DebugInterceptor{logFile: opts.LogFile}

	if opts.Mutex != nil {
		i.mutex = opts.Mutex
	} else {
		i.mutex = &sync.Mutex{}
	}

	// Store the instance in the global cache.
	interceptors.Store(opts.LogFile, i)

	return i, nil
}

func (i *DebugInterceptor) ServerOptions(opts LogOptions) []grpc.ServerOption {
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(i.DebugServerInterceptor(opts)),
		grpc.ChainStreamInterceptor(i.DebugStreamServerInterceptor(opts)),
	}
}

func (i *DebugInterceptor) DialOptions(opts LogOptions) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(i.DebugClientInterceptor(opts)),
		grpc.WithChainStreamInterceptor(i.DebugStreamClientInterceptor(opts)),
	}
}

// Logs all gRPC converations in JSON format.
//
// To enable, call InitDebugInterceptors first in your process main to
// configure the location of the Go file.
func (i *DebugInterceptor) DebugServerInterceptor(opts LogOptions) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (any, error) {
		log := debugInterceptorLogEntry{
			Method:   info.FullMethod,
			Metadata: opts.Metadata,
		}
		i.trackRequest(&log, req)
		if err := i.record(log); err != nil {
			return nil, fmt.Errorf("failed to record GRPC debug log request: %w", err)
		}
		resp, err := handler(ctx, req)
		i.trackResponse(&log, resp)
		i.trackResponseCompleted(&log)
		if e := i.record(log); e != nil {
			return resp, e
		}
		return resp, err
	}
}

// Like debugServerInterceptor but for streaming calls.
func (i *DebugInterceptor) DebugStreamServerInterceptor(opts LogOptions) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ssWrapped := &debugServerStream{
			interceptor:       i,
			method:            info.FullMethod,
			innerServerStream: ss,
			metadata:          opts.Metadata,
		}
		err := handler(srv, ssWrapped)
		return err
	}
}

// Like debugServerInterceptor but for GRPC client connections.
func (i *DebugInterceptor) DebugClientInterceptor(opts LogOptions) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any,
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, gopts ...grpc.CallOption,
	) error {
		// Ignoring weird entries with empty method and nil req and reply.
		if method == "" {
			return invoker(ctx, method, req, reply, cc, gopts...)
		}

		log := debugInterceptorLogEntry{
			Method:    method,
			Metadata:  opts.Metadata,
			Timestamp: time.Now(),
		}
		i.trackRequest(&log, req)
		if err := i.record(log); err != nil {
			return fmt.Errorf("failed to record GRPC debug log request: %w", err)
		}
		err := invoker(ctx, method, req, reply, cc, gopts...)
		if err != nil {
			i.track(&log, err)
		} else {
			i.trackResponse(&log, reply)
		}

		i.trackResponseCompleted(&log)
		if e := i.record(log); e != nil {
			return e
		}
		return err
	}
}

// Like debugClientInterceptor but for streaming calls.
func (i *DebugInterceptor) DebugStreamClientInterceptor(opts LogOptions) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, gopts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, gopts...)

		wrappedStream := &debugClientStream{
			innerClientStream: stream,
			interceptor:       i,
			method:            method,
			metadata:          opts.Metadata,
		}

		return wrappedStream, err
	}
}

func (i *DebugInterceptor) record(log debugInterceptorLogEntry) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	f, err := os.OpenFile(i.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("failed to append GRPC debug logs to file %s: %w", i.logFile, err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(log); err != nil {
		return fmt.Errorf("failed to encode GRPC debug logs: %w", err)
	}
	return nil
}

func (*DebugInterceptor) track(log *debugInterceptorLogEntry, err error) {
	log.Errors = append(log.Errors, err.Error())
}

func (i *DebugInterceptor) trackRequest(log *debugInterceptorLogEntry, req any) {
	j, err := i.transcode(req)
	if err != nil {
		i.track(log, err)
	} else {
		log.Request = j
		log.Progress = "request_started"
	}
}

func (i *DebugInterceptor) trackResponse(log *debugInterceptorLogEntry, resp any) {
	j, err := i.transcode(resp)
	if err != nil {
		i.track(log, err)
	} else {
		log.Response = j
	}
}

func (i *DebugInterceptor) trackResponseCompleted(log *debugInterceptorLogEntry) {
	now := time.Now()
	duration := now.Sub(log.Timestamp)
	log.Progress = "response_completed"
	log.Duration = duration
	log.Timestamp = now
}

func (*DebugInterceptor) transcode(obj any) (json.RawMessage, error) {
	if obj == nil {
		return json.RawMessage("null"), nil
	}

	m, ok := obj.(protoreflect.ProtoMessage)
	if !ok {
		return json.RawMessage("null"),
			fmt.Errorf("failed to decode, expecting protoreflect.ProtoMessage, got %v",
				reflect.TypeOf(obj))
	}

	buf, err := protojson.Marshal(m)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// Wraps grpc.ServerStream with interceptor hooks for SendMsg, RecvMsg.
type debugServerStream struct {
	innerServerStream grpc.ServerStream
	interceptor       *DebugInterceptor
	method            string
	metadata          any
}

func (dss *debugServerStream) errorEntry(err error) debugInterceptorLogEntry {
	return debugInterceptorLogEntry{
		Metadata:  dss.metadata,
		Method:    dss.method,
		Errors:    []string{err.Error()},
		Timestamp: time.Now(),
	}
}

func (dss *debugServerStream) SetHeader(md metadata.MD) error {
	return dss.innerServerStream.SetHeader(md)
}

func (dss *debugServerStream) SendHeader(md metadata.MD) error {
	return dss.innerServerStream.SendHeader(md)
}

func (dss *debugServerStream) SetTrailer(md metadata.MD) {
	dss.innerServerStream.SetTrailer(md)
}

func (dss *debugServerStream) Context() context.Context {
	return dss.innerServerStream.Context()
}

func (dss *debugServerStream) SendMsg(m any) error {
	err := dss.innerServerStream.SendMsg(m)
	if err != nil {
		if e := dss.interceptor.record(dss.errorEntry(err)); e != nil {
			return e
		}
	} else {
		req, err := dss.interceptor.transcode(m)
		if err != nil {
			if e := dss.interceptor.record(dss.errorEntry(err)); e != nil {
				return e
			}
		} else {
			if e := dss.interceptor.record(debugInterceptorLogEntry{
				Metadata:  dss.metadata,
				Method:    dss.method,
				Request:   req,
				Timestamp: time.Now(),
			}); e != nil {
				return e
			}
		}
	}
	return err
}

func (dss *debugServerStream) RecvMsg(m any) error {
	err := dss.innerServerStream.RecvMsg(m)
	if err == io.EOF {
		return err
	} else if err != nil {
		if e := dss.interceptor.record(dss.errorEntry(err)); e != nil {
			return e
		}
	} else {
		resp, err := dss.interceptor.transcode(m)
		if err != nil {
			if e := dss.interceptor.record(dss.errorEntry(err)); e != nil {
				return e
			}
		} else {
			if e := dss.interceptor.record(debugInterceptorLogEntry{
				Method:    dss.method,
				Metadata:  dss.metadata,
				Response:  resp,
				Timestamp: time.Now(),
			}); e != nil {
				return e
			}
		}
	}
	return err
}

var _ grpc.ServerStream = &debugServerStream{}

// Wraps grpc.ClientStream with interceptor hooks for SendMsg, RecvMsg.
type debugClientStream struct {
	innerClientStream grpc.ClientStream
	interceptor       *DebugInterceptor
	method            string
	metadata          any
}

func (d *debugClientStream) errorEntry(err error) debugInterceptorLogEntry {
	return debugInterceptorLogEntry{
		Method:    d.method,
		Metadata:  d.metadata,
		Timestamp: time.Now(),
		Errors:    []string{err.Error()},
	}
}

func (d *debugClientStream) Header() (metadata.MD, error) {
	return d.innerClientStream.Header()
}

func (d *debugClientStream) Trailer() metadata.MD {
	return d.innerClientStream.Trailer()
}

func (d *debugClientStream) CloseSend() error {
	return d.innerClientStream.CloseSend()
}

func (d *debugClientStream) Context() context.Context {
	return d.innerClientStream.Context()
}

func (d *debugClientStream) SendMsg(m any) error {
	err := d.innerClientStream.SendMsg(m)
	if err != nil {
		if e := d.interceptor.record(d.errorEntry(err)); e != nil {
			return e
		}
	} else {
		req, err := d.interceptor.transcode(m)
		if err != nil {
			if e := d.interceptor.record(d.errorEntry(err)); e != nil {
				return e
			}
		} else {
			if e := d.interceptor.record(debugInterceptorLogEntry{
				Method:   d.method,
				Metadata: d.metadata,
				Request:  req,
			}); e != nil {
				return e
			}
		}
	}
	return err
}

func (d *debugClientStream) RecvMsg(m any) error {
	err := d.innerClientStream.RecvMsg(m)
	if err == io.EOF {
		return err
	} else if err != nil {
		if e := d.interceptor.record(d.errorEntry(err)); e != nil {
			return e
		}
	} else {
		resp, err := d.interceptor.transcode(m)
		if err != nil {
			if e := d.interceptor.record(d.errorEntry(err)); e != nil {
				return e
			}
		} else {
			if e := d.interceptor.record(debugInterceptorLogEntry{
				Method:   d.method,
				Metadata: d.metadata,
				Response: resp,
			}); e != nil {
				return e
			}
		}
	}
	return err
}

var _ grpc.ClientStream = &debugClientStream{}
