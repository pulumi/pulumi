package main

import (
	"example.com/pulumi-splat/sdk/go/splat"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		allKeys, err := splat.GetSshKeys(ctx, map[string]interface{}{}, nil)
		if err != nil {
			return err
		}
		var splat0 []string
		for _, val0 := range allKeys.SshKeys {
			splat0 = append(splat0, val0.Name)
		}
		_, err = splat.NewServer(ctx, "main", &splat.ServerArgs{
			SshKeys: splat0,
		})
		if err != nil {
			return err
		}
		return nil
	})
}
