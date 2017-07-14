// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"strconv"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/rpcutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

// hostServer is the server side of the host RPC machinery.
type hostServer struct {
	host   Host       // the host for this RPC server.
	ctx    *Context   // the associated plugin context.
	port   int        // the port the host is listening on.
	cancel chan bool  // a channel that can cancel the server.
	done   chan error // a channel that resolves when the server completes.
}

// newHostServer creates a new host server wired up to the given host and context.
func newHostServer(host Host, ctx *Context) (*hostServer, error) {
	// New up an engine RPC server.
	engine := &hostServer{
		host:   host,
		ctx:    ctx,
		cancel: make(chan bool),
	}

	// Fire up a gRPC server and start listening for incomings.
	port, done, err := rpcutil.Serve(0, engine.cancel, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			lumirpc.RegisterEngineServer(srv, engine)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}

	engine.port = port
	engine.done = done

	return engine, nil
}

// Address returns the address at which the engine's RPC server may be reached.
func (eng *hostServer) Address() string {
	return ":" + strconv.Itoa(eng.port)
}

// Cancel signals that the engine should be terminated, awaits its termination, and returns any errors that result.
func (eng *hostServer) Cancel() error {
	eng.cancel <- true
	return <-eng.done
}

// Log logs a global message in the engine, including errors and warnings.
func (eng *hostServer) Log(ctx context.Context,
	req *lumirpc.LogRequest) (*pbempty.Empty, error) {
	var sev diag.Severity
	switch req.Severity {
	case lumirpc.LogSeverity_DEBUG:
		sev = diag.Debug
	case lumirpc.LogSeverity_INFO:
		sev = diag.Info
	case lumirpc.LogSeverity_WARNING:
		sev = diag.Warning
	case lumirpc.LogSeverity_ERROR:
		sev = diag.Error
	default:
		return nil, errors.Errorf("Unrecognized logging severity: %v", req.Severity)
	}
	eng.host.Log(sev, req.Message)
	return &pbempty.Empty{}, nil
}

// ReadLocation reads the value from a location identified by a token in the current program.
func (eng *hostServer) ReadLocation(ctx context.Context,
	req *lumirpc.ReadLocationRequest) (*pbstruct.Value, error) {
	tok := tokens.Token(req.Token)
	v, err := eng.host.ReadLocation(tok)
	if err != nil {
		return nil, err
	}
	m, known := MarshalPropertyValue(v, MarshalOptions{})
	if !known {
		return nil, errors.Errorf("Location %v contained an unknown computed value", tok)
	}
	return m, nil
}

// ReadLocations reads takes a class or module token and reads all (static) properties belonging to it.
func (eng *hostServer) ReadLocations(ctx context.Context,
	req *lumirpc.ReadLocationsRequest) (*lumirpc.ReadLocationsResponse, error) {
	tok := tokens.Token(req.Token)
	locs, err := eng.host.ReadLocations(tok)
	if err != nil {
		return nil, err
	}
	props, unks := MarshalPropertiesWithUnknowns(locs, MarshalOptions{})
	if len(unks) > 0 {
		return nil, errors.Errorf("Location %v contained %v unknown computed value(s)", tok, len(unks))
	}
	return &lumirpc.ReadLocationsResponse{Properties: props}, nil
}
