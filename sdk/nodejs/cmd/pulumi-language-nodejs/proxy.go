package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	opentracing "github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type monitorProxy struct {
	targetConn    *grpc.ClientConn
	target        pulumirpc.ResourceMonitorClient
	addr          string
	pipeDirectory string
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
	ctx context.Context, targetAddr string, tracingSpan opentracing.Span) (*monitorProxy, <-chan error, error) {

	pipes, err := createPipes()
	if err != nil {
		return nil, nil, err
	}

	conn, err := grpc.Dial(targetAddr, grpc.WithInsecure())
	if err != nil {
		return nil, nil, err
	}

	proxy := &monitorProxy{
		targetConn:    conn,
		target:        pulumirpc.NewResourceMonitorClient(conn),
		pipeDirectory: pipes,
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
		return nil, nil, err
	}

	pipeErrors := make(chan error)
	go proxy.servePipes(ctx, serverCancel, pipeErrors)

	proxy.addr = fmt.Sprintf("127.0.0.1:%d", port)

	allErrors := mergeErrorChannels(serverErrors, pipeErrors)

	return proxy, allErrors, nil
}

func mergeErrorChannels(cs ...<-chan error) <-chan error {
	out := make(chan error)

	for _, c := range cs {
		go func(c <-chan error) {
			for v := range c {
				out <- v
			}
		}(c)
	}
	return out
}

func createPipes() (string, error) {
	dir, err := ioutil.TempDir("", "pulumi-node-pipes")
	if err != nil {
		return "", err
	}

	invokeReqPath, invokeResPath := path.Join(dir, "invoke_req"), path.Join(dir, "invoke_res")

	if err := syscall.Mkfifo(invokeReqPath, 0600); err != nil {
		return "", err
	}
	if err := syscall.Mkfifo(invokeResPath, 0600); err != nil {
		return "", err
	}

	return dir, nil
}

func (p *monitorProxy) servePipes(ctx context.Context, serverCancel chan<- bool, pipeErrors chan<- error) {
	servePipesImpl := func() error {
		pbcodec := encoding.GetCodec(proto.Name)

		defer contract.IgnoreError(os.Remove(p.pipeDirectory))

		invokeReqPath, invokeResPath := path.Join(p.pipeDirectory, "invoke_req"), path.Join(p.pipeDirectory, "invoke_res")
		invokeReqPipe, err := os.OpenFile(invokeReqPath, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(invokeReqPipe)
		defer contract.IgnoreError(os.Remove(invokeReqPath))

		invokeResPipe, err := os.OpenFile(invokeResPath, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer contract.IgnoreClose(invokeResPipe)
		defer contract.IgnoreError(os.Remove(invokeResPath))

		for {
			// read a 4-byte request length
			var reqLen uint32

			logging.V(10).Infoln("Sync invoke: Reading length from request pipe")
			if err := binary.Read(invokeReqPipe, binary.BigEndian, &reqLen); err != nil {
				// This is benign on shutdown.
				if ctx.Err() == context.Canceled {
					// We were asked to gracefully cancel.  Just exit now.
					return nil
				}

				logging.V(10).Infof("Sync invoke: Received error reading length from pipe: %s\n", err)
				return err
			}

			// read the request in full
			reqBytes := make([]byte, reqLen)
			logging.V(10).Infoln("Sync invoke: Reading message from request pipe")
			if _, err := io.ReadFull(invokeReqPipe, reqBytes); err != nil {
				logging.V(10).Infof("Sync invoke: Received error reading message from pipe: %s\n", err)
				return err
			}

			// decode and dispatch the request
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
			resBytes, err := pbcodec.Marshal(res)
			if err != nil {
				logging.V(10).Infof("Sync invoke: Received error marshalling: %s\n", err)
				return err
			}

			// write the 4-byte response length
			logging.V(10).Infoln("Sync invoke: Writing length to request pipe")
			if err := binary.Write(invokeResPipe, binary.BigEndian, uint32(len(resBytes))); err != nil {
				logging.V(10).Infof("Sync invoke: Error writing length to pipe: %s\n", err)
				return err
			}

			// write the response in full
			logging.V(10).Infoln("Sync invoke: Writing message to request pipe")
			if _, err := invokeResPipe.Write(resBytes); err != nil {
				logging.V(10).Infof("Sync invoke: Error writing message to pipe: %s\n", err)
				return err
			}
		}
	}

	// Keep reading and writing from the pipes until we run into an error or are canceled.
	err := servePipesImpl()
	if err != nil {
		pipeErrors <- err
	}

	// once done, let the server know it can shutdown.
	serverCancel <- true
}

func (p *monitorProxy) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
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

func (p *monitorProxy) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest) (*pbempty.Empty, error) {

	return p.target.RegisterResourceOutputs(ctx, req)
}

func (p *monitorProxy) SupportsFeature(ctx context.Context,
	req *pulumirpc.SupportsFeatureRequest) (*pulumirpc.SupportsFeatureResponse, error) {

	return p.target.SupportsFeature(ctx, req)
}
