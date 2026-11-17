package main

import (
	"example.com/pulumi-objectref/sdk/go/v62/objectref/referencedfirst"
	"example.com/pulumi-objectref/sdk/go/v62/objectref/referencingfirst"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		widget, err := referencingfirst.NewWidget(ctx, "widget", &referencingfirst.WidgetArgs{
			Settings: &referencingfirst.WidgetSettingsArgs{
				Name: pulumi.String("widget"),
			},
		})
		if err != nil {
			return err
		}
		_, err = referencingfirst.NewGadget(ctx, "gadget", &referencingfirst.GadgetArgs{
			WidgetRef: &referencingfirst.WidgetRefArgs{
				Widget: widget,
			},
		})
		if err != nil {
			return err
		}
		alpha, err := referencedfirst.NewAlpha(ctx, "alpha", &referencedfirst.AlphaArgs{
			Settings: &referencedfirst.AlphaSettingsArgs{
				Name: pulumi.String("alpha"),
			},
		})
		if err != nil {
			return err
		}
		_, err = referencedfirst.NewBeta(ctx, "beta", &referencedfirst.BetaArgs{
			AlphaRef: &referencedfirst.AlphaRefArgs{
				Alpha: alpha,
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
