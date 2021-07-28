package main

import (
	"encoding/base64"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_ := base64.StdEncoding.EncodeToString([]byte("haha business"))
		_ := strings.Join([]string{
			"haha",
			"business",
		}, "-")
		return nil
	})
}
