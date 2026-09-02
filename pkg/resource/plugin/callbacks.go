// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// CallbacksClient is a pulumirpc.CallbacksClient wrapper that owns the underlying gRPC connection
// so it can be closed when no longer needed.
type CallbacksClient struct {
	pulumirpc.CallbacksClient

	conn *grpc.ClientConn
}

// Close closes the underlying gRPC connection.
func (c *CallbacksClient) Close() error {
	return c.conn.Close()
}

// NewCallbacksClient constructs a CallbacksClient that wraps conn.
func NewCallbacksClient(conn *grpc.ClientConn) *CallbacksClient {
	return &CallbacksClient{
		CallbacksClient: pulumirpc.NewCallbacksClient(conn),
		conn:            conn,
	}
}

// CallbacksClientCache caches CallbacksClient instances by their gRPC target address. It is safe
// for concurrent use. Consumers that need to invoke a language-side callback (resource transforms,
// resource hooks, the builtin Stash reducer, etc.) share a single cache per plugin.Context so that
// only one connection is opened per callback server target.
type CallbacksClientCache struct {
	lock     sync.Mutex
	clients  map[string]*CallbacksClient
	dialOpts func(pluginInfo any) []grpc.DialOption
}

// NewCallbacksClientCache creates an empty cache. dialOpts, if non-nil, is invoked to provide
// additional per-connection gRPC dial options (mirroring plugin.Context.DialOptions).
func NewCallbacksClientCache(dialOpts func(pluginInfo any) []grpc.DialOption) *CallbacksClientCache {
	return &CallbacksClientCache{
		clients:  map[string]*CallbacksClient{},
		dialOpts: dialOpts,
	}
}

// Get returns the cached client for target, opening a new gRPC connection on first use.
func (c *CallbacksClientCache) Get(target string) (*CallbacksClient, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if client, ok := c.clients[target]; ok {
		return client, nil
	}

	dialOpts := append(
		rpcutil.TracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)
	if c.dialOpts != nil {
		dialOpts = append(dialOpts, c.dialOpts(map[string]any{
			"mode": "client",
			"kind": "callbacks",
		})...)
	}

	conn, err := grpc.NewClient(target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dialing callback server at %s: %w", target, err)
	}

	client := NewCallbacksClient(conn)
	c.clients[target] = client
	return client, nil
}

// Close closes every cached connection. Subsequent calls to Get will open fresh connections.
func (c *CallbacksClientCache) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var errs []error
	for _, client := range c.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	c.clients = map[string]*CallbacksClient{}
	return errors.Join(errs...)
}
