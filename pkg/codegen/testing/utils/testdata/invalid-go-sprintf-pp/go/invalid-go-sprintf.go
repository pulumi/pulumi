package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// example
		_, err := appsv1.NewDeployment(ctx, "argocd_serverDeployment", &appsv1.DeploymentArgs{
			ApiVersion: pulumi.String("apps/v1"),
			Kind:       pulumi.String("Deployment"),
			Metadata: &metav1.ObjectMetaArgs{
				Labels: pulumi.StringMap{
					"app.kubernetes.io/component": pulumi.String("server"),
					"aws:region":                  pulumi.String("us-west-2"),
					"key%percent":                 pulumi.String("percent"),
					"key...ellipse":               pulumi.String("ellipse"),
					"key{bracket":                 pulumi.String("bracket"),
					"key}bracket":                 pulumi.String("bracket"),
					"key*asterix":                 pulumi.String("asterix"),
					"key?question":                pulumi.String("question"),
					"key,comma":                   pulumi.String("comma"),
					"key&&and":                    pulumi.String("and"),
					"key||or":                     pulumi.String("or"),
					"key!not":                     pulumi.String("not"),
					"key=>geq":                    pulumi.String("geq"),
					"key==eq":                     pulumi.String("equal"),
				},
				Name: pulumi.String("argocd-server"),
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
