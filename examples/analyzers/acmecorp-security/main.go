// Copyright 2016 Pulumi, Inc. All rights reserved.

package main

import (
	"fmt"
	"os"

	pbstruct "github.com/golang/protobuf/ptypes/struct"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/util/rpcutil"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func main() {
	// Fire up a gRPC server, letting the kernel choose a free port for us.
	port, done, err := rpcutil.Serve(0, []func(*grpc.Server) error{
		func(srv *grpc.Server) error {
			cocorpc.RegisterAnalyzerServer(srv, &analyzer{})
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

// analyzer implements the suite of ACMECorp security checks.
type analyzer struct {
}

func (a *analyzer) Analyze(ctx context.Context,
	req *cocorpc.AnalyzeRequest) (*cocorpc.AnalyzeResponse, error) {
	// This is intentionally left blank; there are no project-wide checks yet.
	return &cocorpc.AnalyzeResponse{}, nil
}

func (a *analyzer) AnalyzeResource(ctx context.Context,
	req *cocorpc.AnalyzeResourceRequest) (*cocorpc.AnalyzeResourceResponse, error) {
	// Switch on the type to perform some simple checks.
	var failures []*cocorpc.AnalyzeResourceFailure
	t := req.GetType()
	switch t {
	case "aws:ec2/instance:Instance":
		failures = a.analyzeAWSEC2Instance(req.GetProperties())
	}
	return &cocorpc.AnalyzeResourceResponse{failures}, nil
}

func (a *analyzer) analyzeAWSEC2Instance(bag *pbstruct.Struct) []*cocorpc.AnalyzeResourceFailure {
	props := resource.UnmarshalProperties(bag)
	image := props["imageId"]
	// TODO: do a real check.  For now, we make something up.
	return []*cocorpc.AnalyzeResourceFailure{
		{
			Property: "imageId",
			Reason: fmt.Sprintf(
				"AMI '%v' has known unpatched security vulnerabilities (CVE-2017-6410, CVE-2017-2326)", image),
		},
	}
}
