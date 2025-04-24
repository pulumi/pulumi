package main

import (
	"os"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		os.WriteFile("ready", []byte("✅"), 0644)
		time.Sleep(10 * time.Second)
		return nil
	})
}
