package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		requiredString := cfg.Require("requiredString")
		requiredInt := cfg.RequireInt("requiredInt")
		requiredFloat := cfg.RequireFloat64("requiredFloat")
		requiredBool := cfg.RequireBool("requiredBool")
		requiredAny := cfg.RequireObject("requiredAny")
		optionalString := "defaultStringValue"
		if param := cfg.Get("optionalString"); param != "" {
			optionalString = param
		}
		optionalInt := 42
		if param := cfg.GetInt("optionalInt"); param != 0 {
			optionalInt = param
		}
		optionalFloat := float64(3.14)
		if param := cfg.GetFloat64("optionalFloat"); param != 0 {
			optionalFloat = param
		}
		optionalBool := true
		if param := cfg.GetBool("optionalBool"); param {
			optionalBool = param
		}
		optionalAny := map[string]interface{}{
			"key": "value",
		}
		if param := cfg.GetObject("optionalAny"); param != nil {
			optionalAny = param
		}
		return nil
	})
}
