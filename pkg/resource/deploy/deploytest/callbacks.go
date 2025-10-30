package deploytest

import deploytest "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"

type CallbackServer = deploytest.CallbackServer

func NewCallbacksServer() (*CallbackServer, error) {
	return deploytest.NewCallbacksServer()
}

