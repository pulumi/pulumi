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
	"github.com/pulumi/pulumi/pkg/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type monitorProxy struct {
	targetConn *grpc.ClientConn
	target     pulumirpc.ResourceMonitorClient

	addr   string
	cancel chan bool
	done   chan error

	pipeDirectory string
}

func newMonitorProxy(targetAddr string, tracingSpan opentracing.Span) (*monitorProxy, error) {
	pipes, err := createPipes()
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(targetAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	proxy := &monitorProxy{
		targetConn:    conn,
		target:        pulumirpc.NewResourceMonitorClient(conn),
		cancel:        make(chan bool),
		pipeDirectory: pipes,
	}

	port, done, err := rpcutil.Serve(0, proxy.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterResourceMonitorServer(srv, proxy)
			return nil
		},
	}, tracingSpan)
	if err != nil {
		return nil, err
	}

	go proxy.servePipes(context.TODO())

	proxy.addr = fmt.Sprintf("127.0.0.1:%d", port)
	proxy.done = done

	return proxy, nil
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

func (p *monitorProxy) servePipes(ctx context.Context) {
	pbcodec := encoding.GetCodec(proto.Name)

	invokeReqPath, invokeResPath := path.Join(p.pipeDirectory, "invoke_req"), path.Join(p.pipeDirectory, "invoke_res")
	invokeReqPipe, err := os.OpenFile(invokeReqPath, os.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer contract.IgnoreClose(invokeReqPipe)

	invokeResPipe, err := os.OpenFile(invokeResPath, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer contract.IgnoreClose(invokeResPipe)

	for {
		// read a 4-byte request length
		var reqLen uint32
		if err := binary.Read(invokeReqPipe, binary.BigEndian, &reqLen); err != nil {
			return
		}

		// read the request in full
		reqBytes := make([]byte, reqLen)
		if _, err := io.ReadFull(invokeReqPipe, reqBytes); err != nil {
			return
		}

		// decode and dispatch the request
		var req pulumirpc.InvokeRequest
		if err := pbcodec.Unmarshal(reqBytes, &req); err != nil {
			return
		}
		res, err := p.Invoke(ctx, &req)
		if err != nil {
			return
		}

		// encode the response
		resBytes, err := pbcodec.Marshal(res)
		if err != nil {
			return
		}
		// write the 4-byte response length
		if err := binary.Write(invokeResPipe, binary.BigEndian, uint32(len(resBytes))); err != nil {
			return
		}
		// write the response in full
		if _, err := invokeResPipe.Write(resBytes); err != nil {
			return
		}
	}
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
