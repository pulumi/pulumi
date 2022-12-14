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
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// Healthcheck will poll the address at duration intervals and then call cancel once it reports unhealthy
func Healthcheck(context context.Context, addr string, duration time.Duration, cancel context.CancelFunc) error {
	conn, err := grpc.Dial(
		addr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(OpenTracingStreamClientInterceptor()),
		GrpcChannelOptions(),
	)
	if err != nil {
		return err
	}

	health := healthpb.NewHealthClient(conn)

	go func() {
		timer := time.NewTimer(duration)
		for {
			select {
			case <-timer.C:
				// Time to check the health status
				req := &healthpb.HealthCheckRequest{}
				_, err := health.Check(context, req)
				// Treat unimplemented as ok. We just want to check the engine looks alive.
				if err != nil && status.Code(err) != codes.Unimplemented {
					// Any other err should trigger the cancellation
					cancel()
					return
				}
				// TODO Should we cancel if the response is HealthCheckResponse_NOT_SERVING?

				// Restart the timer and wait again
				timer.Reset(duration)
			case <-context.Done():
				// Context is done, time to quit
				return
			}
		}
	}()

	return nil
}
