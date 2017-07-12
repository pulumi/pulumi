// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package main

import (
	"github.com/pulumi/lumi/pkg/resource/provider"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

func main() {
	// Create a new resurce provider server and listen for and serve incoming connections.
	if err := provider.Main(func(host *provider.HostClient) (lumirpc.ResourceProviderServer, error) {
		return NewProvider(host)
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
