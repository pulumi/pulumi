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

package main

import (
	"context"
	"encoding/binary"
	"io"

	pbempty "github.com/golang/protobuf/ptypes/empty"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// pipes is the platform agnostic abstraction over a pair of channels we can read and write binary
// data over. It is provided through the `createPipes` functions provided in `proxy_unix.go` (where
// it is implemented on top of fifo files), and in `proxy_windows.go` (where it is implemented on
// top of named pipes).
type pipes interface {
	// The directory containing the two streams to read and write from.  This will be passed to the
	// nodejs process so it can connect to our read and writes streams for communication.
	directory() string

	// Attempt to create and connect to the read and write streams.
	connect() error

	// The stream that we will use to read in requests send to us by the nodejs process.
	reader() io.Reader

	// The stream we will write responses back to the nodejs process with.
	writer() io.Writer

	// called when we're done with the pipes and want to clean up any os resources we may have
	// allocated (for example, actual files and directories on disk).
	shutdown()
}

func createAndServePipes(ctx context.Context, target pulumirpc.ResourceMonitorClient) (pipes, chan error, error) {
	pipes, err := createPipes()
	if err != nil {
		return nil, nil, err
	}

	pipesDone := servePipes(ctx, pipes, target)
	return pipes, pipesDone, nil
}

func servePipes(ctx context.Context, pipes pipes, target pulumirpc.ResourceMonitorClient) chan error {
	done := make(chan error)

	go func() {
		// Keep reading and writing from the pipes until we run into an error or are canceled.
		err := func() error {
			pbcodec := encoding.GetCodec(proto.Name)

			err := pipes.connect()
			if err != nil {
				logging.V(10).Infof("Sync invoke: Error connecting to pipes: %s\n", err)
				return err
			}

			for {
				// read a 4-byte request length
				logging.V(10).Infoln("Sync invoke: Reading length from request pipe")
				var reqLen uint32
				if err := binary.Read(pipes.reader(), binary.BigEndian, &reqLen); err != nil {
					// This is benign on shutdown.
					if err == io.EOF {
						// We were asked to gracefully cancel.  Just exit now.
						logging.V(10).Infof("Sync invoke: Gracefully shutting down")
						return nil
					}

					logging.V(10).Infof("Sync invoke: Received error reading length from pipe: %s\n", err)
					return err
				}

				// read the request in full
				logging.V(10).Infoln("Sync invoke: Reading message from request pipe")
				reqBytes := make([]byte, reqLen)
				if _, err := io.ReadFull(pipes.reader(), reqBytes); err != nil {
					logging.V(10).Infof("Sync invoke: Received error reading message from pipe: %s\n", err)
					return err
				}

				// decode and dispatch the request
				logging.V(10).Infof("Sync invoke: Unmarshalling request")
				var req pulumirpc.InvokeRequest
				if err := pbcodec.Unmarshal(reqBytes, &req); err != nil {
					logging.V(10).Infof("Sync invoke: Received error reading full from pipe: %s\n", err)
					return err
				}

				logging.V(10).Infof("Sync invoke: Invoking: %s", req.GetTok())
				res, err := target.Invoke(ctx, &req)

				// Unfortunately, `monitor.Invoke` can return errors for non-exceptional cases. This
				// can happen, for example, if the underlying provider call ends up returning an
				// error itself.  A common case of this is the aws-provider which will often return
				// errors like:
				//
				//  aws:ec2/getRouteTable:getRouteTable: Your query returned no results.
				//
				// This is not an error in the sense that something has gone terribly wrong and we
				// need to shutdown.  Rather, it's just a validation issue that needs to be passed
				// back to the user program to handle.
				//
				// In the async codepath, this just returns as a grpc error which is thrown in the
				// sdk code (and can be handled by the user). Unfortunately, our sync-rpc pipes have
				// no way to send either an err or a success result.  They can only send success,
				// and only *terminate* on error.
				//
				// So, to workaround this issue, we abuse the 'success result' slightly.
				// Specifically, we take advantage of the fact that the success-result contains a
				// list of 'failures' to indicate if 'Check' reported any problems.  We just stash
				// this error into that array, knowing that the SDK will see it and immediately
				// throw, just like it would have done for an RPC error in the normal async case.
				if err != nil {
					logging.V(10).Infof("Sync invoke: Received error invoking: %s\n", err)
					logging.V(10).Infof("Sync invoke: Converting error to response.\n")
					if res == nil {
						res = &pulumirpc.InvokeResponse{}
					}

					res.Failures = append(res.Failures, &pulumirpc.CheckFailure{
						Property: "",
						Reason:   err.Error(),
					})
				}

				// encode the response
				logging.V(10).Infof("Sync invoke: Marshalling response")
				resBytes, err := pbcodec.Marshal(res)
				if err != nil {
					logging.V(10).Infof("Sync invoke: Received error marshalling: %s\n", err)
					return err
				}

				// write the 4-byte response length
				logging.V(10).Infoln("Sync invoke: Writing length to request pipe")
				if err := binary.Write(pipes.writer(), binary.BigEndian, uint32(len(resBytes))); err != nil {
					logging.V(10).Infof("Sync invoke: Error writing length to pipe: %s\n", err)
					return err
				}

				// write the response in full
				logging.V(10).Infoln("Sync invoke: Writing message to request pipe")
				if _, err := pipes.writer().Write(resBytes); err != nil {
					logging.V(10).Infof("Sync invoke: Error writing message to pipe: %s\n", err)
					return err
				}
			}
		}()

		// Signal our caller that we're done.
		done <- err
		close(done)

		// cleanup any resources the pipes were holding onto.
		pipes.shutdown()
	}()

	return done
}

// Forward all resource monitor calls that we're serving to nodejs back to the engine to actually
// perform.

type monitorProxy struct {
	target pulumirpc.ResourceMonitorClient
}

func (p *monitorProxy) Invoke(
	ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return p.target.Invoke(ctx, req)
}

func (p *monitorProxy) StreamInvoke(
	req *pulumirpc.InvokeRequest, server pulumirpc.ResourceMonitor_StreamInvokeServer) error {

	client, err := p.target.StreamInvoke(context.Background(), req)
	if err != nil {
		return err
	}

	for {
		in, err := client.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := server.Send(in); err != nil {
			return err
		}
	}
}

func (p *monitorProxy) Call(
	ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return p.target.Call(ctx, req)
}

func (p *monitorProxy) ReadResource(
	ctx context.Context, req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error) {
	return p.target.ReadResource(ctx, req)
}

func (p *monitorProxy) RegisterResource(
	ctx context.Context, req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {
	return p.target.RegisterResource(ctx, req)
}

func (p *monitorProxy) RegisterResourceOutputs(
	ctx context.Context, req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {
	return p.target.RegisterResourceOutputs(ctx, req)
}

func (p *monitorProxy) SupportsFeature(
	ctx context.Context, req *pulumirpc.SupportsFeatureRequest) (*pulumirpc.SupportsFeatureResponse, error) {
	return p.target.SupportsFeature(ctx, req)
}
