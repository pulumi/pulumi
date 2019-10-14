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
	"fmt"
	"io"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	opentracing "github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type monitorProxy struct {
	target pulumirpc.ResourceMonitorClient
	addr   string
	pipes  pipes
}

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

// When talking to the nodejs runtime we have three parties involved:
//
//  Engine Monitor <==> Language Host (this code) <==> NodeJS sdk runtime.
//
// Instead of having the NodeJS sdk runtime communicating directly with the Engine Monitor, we
// instead have it communicate with us and we send all those messages to the real engine monitor
// itself.  We do that by having ourselves launch our own grpc monitor server and passing the
// address of it to the NodeJS runtime.  As far as the NodeJS sdk runtime is concerned, it is
// communicating directly with the engine.
//
// When NodeJS then communicates back with us over our server, we then forward the messages
// along untouched to the Engine Monitor.  However, we also open an additional *non-grpc*
// channel to allow the sdk runtime to send us messages on.  Specifically, this non-grpc channel
// is used entirely to allow the sdk runtime to make 'invoke' calls in a synchronous fashion.
// This is accomplished by avoiding grpc entirely (which has no facility for synchronous rpc
// calls), and instead operating over a pair of files coordinated between us and the sdk
// runtime. One file is used by it to send us messages, and one file is used by us to send
// messages back.  Because these are just files, nodejs natively supports allowing both sides to
// read and write from each synchronously.
//
// When we receive the sync-invoke messages from the nodejs sdk we deserialize things off of the
// file and then make a synchronous call to the real engine `invoke` monitor endpoint.  Unlike
// nodejs, we have no problem calling this synchronously, and can block until we get the
// response which we can then synchronously send to nodejs.
func newMonitorProxy(
	ctx context.Context, responseChannel chan<- *pulumirpc.RunResponse,
	targetAddr string, tracingSpan opentracing.Span) (*monitorProxy, error) {

	pipes, err := createPipes()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(targetAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	proxy := &monitorProxy{
		target: pulumirpc.NewResourceMonitorClient(conn),
		pipes:  pipes,
	}

	// Channel to control the server lifetime.  The pipe reading code will ask the server to
	// shutdown once it has finished reading/writing from the pipes for any reason.
	serverCancel := make(chan bool)
	port, serverErrors, err := rpcutil.Serve(0, serverCancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, proxy)
			return nil
		},
	}, tracingSpan)
	if err != nil {
		return nil, err
	}

	proxy.addr = fmt.Sprintf("127.0.0.1:%d", port)

	// Listen for errors from the server and push them to the singular error stream if we receive one.
	go func() {
		err := <-serverErrors
		if err != nil {
			responseChannel <- &pulumirpc.RunResponse{Error: err.Error()}
		}
	}()

	// Now, kick off a goroutine to actually read and write from the pipes.  Any errors should be
	// reported to `responseChannel`.  When complete, `serverCancel` should be called to let the
	// server know it can shutdown gracefully.
	go proxy.servePipes(ctx, responseChannel, serverCancel)

	return proxy, nil
}

func (p *monitorProxy) servePipes(
	ctx context.Context, resultChannel chan<- *pulumirpc.RunResponse, serverCancel chan<- bool) {

	// Once we're done using the pipes clean them up so we don't leave anything around in the user
	// file system.  Also let the server know it can shutdown gracefully.
	defer p.pipes.shutdown()
	defer func() { serverCancel <- true }()

	// Keep reading and writing from the pipes until we run into an error or are canceled.
	err := func() error {
		pbcodec := encoding.GetCodec(proto.Name)

		err:= p.pipes.connect();
		if err != nil {
			logging.V(10).Infof("Sync invoke: Error connecting to pipes: %s\n", err)
			return err
		}

		for {
			// read a 4-byte request length
			logging.V(10).Infoln("Sync invoke: Reading length from request pipe")
			var reqLen uint32
			if err := binary.Read(p.pipes.reader(), binary.BigEndian, &reqLen); err != nil {
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
			if _, err := io.ReadFull(p.pipes.reader(), reqBytes); err != nil {
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
			res, err := p.Invoke(ctx, &req)
			if err != nil {
				logging.V(10).Infof("Sync invoke: Received error invoking: %s\n", err)
				return err
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
			if err := binary.Write(p.pipes.writer(), binary.BigEndian, uint32(len(resBytes))); err != nil {
				logging.V(10).Infof("Sync invoke: Error writing length to pipe: %s\n", err)
				return err
			}

			// write the response in full
			logging.V(10).Infoln("Sync invoke: Writing message to request pipe")
			if _, err := p.pipes.writer().Write(resBytes); err != nil {
				logging.V(10).Infof("Sync invoke: Error writing message to pipe: %s\n", err)
				return err
			}
		}
	}()

	if err != nil {
		// If we received an error serving pipes, then notify our caller so they can pass it along.
		resultChannel <- &pulumirpc.RunResponse{Error: err.Error()}
	}
}

// Forward all resource monitor calls that we're serving to nodejs back to the engine to actually
// perform.

func (p *monitorProxy) Invoke(
	ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {

	return p.target.Invoke(ctx, req)
}

func (p *monitorProxy) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error) {

	return p.target.ReadResource(ctx, req)
}

func (p *monitorProxy) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error) {

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
