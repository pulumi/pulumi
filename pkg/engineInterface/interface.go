package engineInterface

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"
	"google.golang.org/grpc"
)

type EngineServer interface {
	pulumirpc.EngineServer

	// Address returns the address at which the engine's RPC server may be reached.
	Address() string

	// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
	Cancel() error

	// Done awaits the engines termination, and returns any errors that result.
	Done() error
}

func Start(ctx context.Context) (EngineServer, error) {
	// New up an engine RPC server.
	engine := &engineServer{
		ctx:    ctx,
		cancel: make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, engine.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			pulumirpc.RegisterEngineServer(srv, engine)
			return nil
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	engine.addr = fmt.Sprintf("127.0.0.1:%d", port)
	engine.done = done

	return engine, nil
}

// engineServer is the server side of the engine RPC machinery.
type engineServer struct {
	ctx    context.Context
	cancel chan bool
	done   chan error
	addr   string
}

func (eng *engineServer) Address() string {
	return eng.addr
}

func (eng *engineServer) Cancel() error {
	eng.cancel <- true
	return <-eng.done
}

func (eng *engineServer) Done() error {
	return <-eng.done
}

func (eng *engineServer) About(ctx context.Context, req *pulumirpc.AboutRequest) (*pulumirpc.AboutResponse, error) {
	return getAbout(ctx, req.TransitiveDependencies, req.Stack)
}
