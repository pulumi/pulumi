package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := appsv1.NewDeployment(ctx, "argocd_serverDeployment", &appsv1.DeploymentArgs{
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			Metadata: &*metav1.ObjectMetaArgs{
				Name: "argocd-server",
			},
			Spec: &*appsv1.DeploymentSpecArgs{
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: map[string]pulumi.String{
						"app": pulumi.String("server"),
					},
				},
				Replicas: 1,
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &*metav1.ObjectMetaArgs{
						Labels: map[string]pulumi.String{
							"app": pulumi.String("server"),
						},
					},
					Spec: &*corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String("nginx"),
								Image: "nginx",
								Ports: []corev1.ContainerPortArgs{
									{
										ContainerPort: pulumi.Int(80),
									},
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
