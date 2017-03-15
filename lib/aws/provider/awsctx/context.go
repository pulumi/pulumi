// Copyright 2017 Pulumi, Inc. All rights reserved.

package awsctx

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pulumi/coconut/pkg/util/contract"
)

// Context represents state shared amongst all parties in this process.  In particular, it wraps an AWS session
// object and offers convenient wrappers for creating connections to the various sub-services (EC2, S3, etc).
type Context struct {
	sess *session.Session
	ec2  *ec2.EC2
}

func New() (*Context, error) {
	// Create an AWS session; note that this is safe to share among many operations.
	// TODO: consider verifying credentials, region, etc. here.
	// TODO: currently we just inherit the standard AWS SDK credentials logic; eventually we will want more
	//     flexibility, I assume, including possibly reading from configuration dynamically.
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	contract.Assert(sess != nil)

	// Allocate a new global context with this session; note that all other connections are lazily allocated.
	return &Context{
		sess: sess,
	}, nil
}

func (ctx *Context) EC2() *ec2.EC2 {
	contract.Assert(ctx.sess != nil)
	if ctx.ec2 == nil {
		ctx.ec2 = ec2.New(ctx.sess)
	}
	return ctx.ec2
}

// Request manufactures a standard Golang context object for a request within this overall AWS context.
func (ctx *Context) Request() context.Context {
	// TODO: unify this with the gRPC context; this will be easier once gRPC moves to the standard Golang context.
	return context.Background()
}
