// Copyright 2017 Pulumi, Inc. All rights reserved.

package main

import (
	"os"

	"github.com/fission/fission/controller/client"
	"github.com/pulumi/coconut/pkg/util/contract"
)

type Context struct {
	Fission *client.Client // the Fission controller client.
}

func NewContext() *Context {
	// TODO[pulumi/coconut#117]: fetch the client from config rather than environment variables.
	url := os.Getenv("FISSION_URL")
	contract.Assertf(url != "", "Missing FISSION_URL environment variable")
	return &Context{Fission: client.MakeClient(url)}
}
