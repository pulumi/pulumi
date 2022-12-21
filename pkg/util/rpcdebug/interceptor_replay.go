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
	"fmt"
	"reflect"

	"google.golang.org/grpc"
)

type ReplayInterceptorOptions struct{}

type ReplayInterceptor struct{}

func NewReplayInterceptor(opts ReplayInterceptorOptions) (*ReplayInterceptor, error) {
	return &ReplayInterceptor{}, nil
}

func (i *ReplayInterceptor) ClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, gopts ...grpc.CallOption) error {
		panic(fmt.Sprintf("ReplayInterceptor caught client call %v", reflect.TypeOf(reply)))
	}
}

func (i *ReplayInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string,
		streamer grpc.Streamer, gopts ...grpc.CallOption) (grpc.ClientStream, error) {
		panic("ReplayInterceptor caught client stream call")
	}
}

func (i *ReplayInterceptor) DialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithChainUnaryInterceptor(i.ClientInterceptor()),
		grpc.WithChainStreamInterceptor(i.StreamClientInterceptor()),
	}
}
