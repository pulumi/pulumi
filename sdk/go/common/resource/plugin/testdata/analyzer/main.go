package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"google.golang.org/grpc"
)

func main() {
	// Bootup a policy plugin but first assert that the config is what we expect

	config := os.Getenv("PULUMI_CONFIG")
	var actual map[string]interface{}
	if err := json.Unmarshal([]byte(config), &actual); err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
	expect := map[string]interface{}{
		"test-project:bool":   "true",
		"test-project:float":  "1.5",
		"test-project:string": "hello",
		"test-project:obj":    "{\"key\":\"value\"}",
	}
	if !reflect.DeepEqual(actual, expect) {
		fmt.Printf("fatal: expected config to be %v, got %v\n", expect, actual)
		os.Exit(1)
	}

	var cancelChannel chan bool
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancelChannel,
		Init: func(srv *grpc.Server) error {
			// pulumirpc.RegisterResourceProviderServer(srv, prov)
			return nil
		},
		Options: rpcutil.OpenTracingServerInterceptorOptions(nil),
	})
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%d\n", handle.Port)

	if err := <-handle.Done; err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}
