// Copyright 2016-2018, Pulumi Corporation.
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
	"io"
	"os"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// metadataReaderWriter satisfies both the opentracing.TextMapReader and
// opentracing.TextMapWriter interfaces.
type metadataReaderWriter struct {
	metadata.MD
}

func (w metadataReaderWriter) Set(key, val string) {
	// The GRPC HPACK implementation rejects any uppercase keys here.
	//
	// As such, since the HTTP_HEADERS format is case-insensitive anyway, we
	// blindly lowercase the key (which is guaranteed to work in the
	// Inject/Extract sense per the OpenTracing spec).
	key = strings.ToLower(key)
	w.MD[key] = append(w.MD[key], val)
}

func (w metadataReaderWriter) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range w.MD {
		for _, v := range vals {
			if err := handler(k, v); err != nil {
				return err
			}
		}
	}

	return nil
}

// OpenTracingServerInterceptor provides a default gRPC server interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingServerInterceptor(parentSpan opentracing.Span, options ...otgrpc.Option) grpc.UnaryServerInterceptor {
	// Log full payloads along with trace spans
	options = append(options, otgrpc.LogPayloads())

	tracingInterceptor := otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer(), options...)
	if parentSpan == nil {
		return tracingInterceptor
	}
	spanContext := parentSpan.Context()
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}
		carrier := metadataReaderWriter{md}
		_, err := opentracing.GlobalTracer().Extract(opentracing.HTTPHeaders, carrier)
		if err == opentracing.ErrSpanContextNotFound {
			contract.IgnoreError(opentracing.GlobalTracer().Inject(spanContext, opentracing.HTTPHeaders, carrier))
		}
		return tracingInterceptor(ctx, req, info, handler)
	}
}

// OpenTracingClientInterceptor provides a default gRPC client interceptor for emitting tracing to the global
// OpenTracing tracer.
func OpenTracingClientInterceptor(options ...otgrpc.Option) grpc.UnaryClientInterceptor {
	options = append(options,
		// Log full payloads along with trace spans
		otgrpc.LogPayloads(),
		// Do not trace calls to the empty method
		otgrpc.IncludingSpans(func(_ opentracing.SpanContext, method string, _, _ interface{}) bool {
			return method != ""
		}))
	return otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer(), options...)
}

type grpcRequestLog struct {
	FullMethod string          `json:"fullMethod"`
	Request    json.RawMessage `json:"request"`
	Response   json.RawMessage `json:"response"`
	Errors     []string        `json:"errors,omitempty"`
}

func (l *grpcRequestLog) transcode(m proto.Message) (json.RawMessage, error) {
	jsonSer := jsonpb.Marshaler{
		Indent: "  ",
	}
	buf := bytes.Buffer{}
	if err := jsonSer.Marshal(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (l *grpcRequestLog) setRequest(m proto.Message) error {
	j, err := l.transcode(m)
	if err != nil {
		return err
	}
	l.Request = j
	return nil
}

func (l *grpcRequestLog) setResponse(m proto.Message) error {
	j, err := l.transcode(m)
	if err != nil {
		return err
	}
	l.Response = j
	return nil
}

func (l *grpcRequestLog) addError(e error) {
	l.Errors = append(l.Errors, e.Error())
}

// LoggingServerInterceptor provides a gRPC server interceptor for
// logging gRPC payloads to the given writer.
func LoggingServerInterceptor(writer io.Writer) grpc.UnaryServerInterceptor {
	mu := &sync.Mutex{}
	jw := json.NewEncoder(writer)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		log := grpcRequestLog{FullMethod: info.FullMethod}
		if err := log.setRequest(req.(proto.Message)); err != nil {
			log.addError(err)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			log.addError(err)
		} else if err := log.setResponse(resp.(proto.Message)); err != nil {
			log.addError(err)
		}
		mu.Lock()
		json.NewEncoder(os.Stderr).Encode(log)
		if err := jw.Encode(log); err != nil {
			panic(err)
		}
		mu.Unlock()
		return resp, err
	}
}
