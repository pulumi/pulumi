package main

import (
	"google.golang.org/grpc"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type CallbacksClient struct {
	pulumirpc.CallbacksClient

	conn *grpc.ClientConn
}

func (c *CallbacksClient) Close() error {
	return c.conn.Close()
}

func NewCallbacksClient(conn *grpc.ClientConn) *CallbacksClient {
	return &CallbacksClient{
		CallbacksClient: pulumirpc.NewCallbacksClient(conn),
		conn:            conn,
	}
}
