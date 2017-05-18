// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/rpcutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	// Fire up a gRPC server, letting the kernel choose a free port for us.
	port, done, err := rpcutil.Serve(0, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			lumirpc.RegisterAnalyzerServer(srv, &analyzer{})
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(-1)
	}

	// The resource provider protocol requires that we now write out the port we have chosen to listen on.
	fmt.Printf("%d\n", port)

	// Finally, wait for the server to stop serving.
	if err := <-done; err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
	}
}

// analyzer implements the suite of infosec checks.
type analyzer struct {
}

func (a *analyzer) Analyze(ctx context.Context,
	req *lumirpc.AnalyzeRequest) (*lumirpc.AnalyzeResponse, error) {
	// This is intentionally left blank; there are no project-wide checks yet.
	return &lumirpc.AnalyzeResponse{}, nil
}

func (a *analyzer) AnalyzeResource(ctx context.Context,
	req *lumirpc.AnalyzeResourceRequest) (*lumirpc.AnalyzeResourceResponse, error) {
	// Switch on the type to perform some simple checks.
	var failures []*lumirpc.AnalyzeResourceFailure
	t := req.GetType()
	switch t {
	case "aws:ec2/instance:Instance":
		failures = a.analyzeAWSEC2Instance(req.GetProperties())
	}
	return &lumirpc.AnalyzeResourceResponse{Failures: failures}, nil
}

func (a *analyzer) analyzeAWSEC2Instance(bag *pbstruct.Struct) []*lumirpc.AnalyzeResourceFailure {
	props := resource.UnmarshalProperties(bag)
	image := props["imageId"]
	// TODO: do a real check.  For now, we make something up.
	return []*lumirpc.AnalyzeResourceFailure{
		{
			Property: "imageId",
			Reason: fmt.Sprintf(
				"AMI '%v' has known unpatched security vulnerabilities (CVE-2017-6410, CVE-2017-2326)", image),
		},
	}
}
