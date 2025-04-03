package main

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		bar, err := corev1.NewPod(ctx, "bar", &corev1.PodArgs{
			ApiVersion: "v1",
			Metadata: &*metav1.ObjectMetaArgs{
				Namespace: "foo",
				Name:      "bar",
				Labels: map[string]pulumi.String{
					"app.kubernetes.io/name":    pulumi.String("cilium-agent"),
					"app.kubernetes.io/part-of": pulumi.String("cilium"),
					"k8s-app":                   pulumi.String("cilium"),
				},
			},
			Spec: &*corev1.PodSpecArgs{
				Containers: corev1.ContainerArray{
					&corev1.ContainerArgs{
						Name:  pulumi.String("nginx"),
						Image: "nginx:1.14-alpine",
						Ports: []corev1.ContainerPortArgs{
							{
								ContainerPort: pulumi.Int(80),
							},
						},
						Resources: &*corev1.ResourceRequirementsArgs{
							Limits: map[string]pulumi.String{
								"memory": pulumi.String("20Mi"),
								"cpu":    pulumi.String("0.2"),
							},
						},
					},
					&corev1.ContainerArgs{
						Name:  pulumi.String("nginx2"),
						Image: "nginx:1.14-alpine",
						Ports: []corev1.ContainerPortArgs{
							{
								ContainerPort: pulumi.Int(80),
							},
						},
						Resources: &*corev1.ResourceRequirementsArgs{
							Limits: map[string]pulumi.String{
								"memory": pulumi.String("20Mi"),
								"cpu":    pulumi.String("0.2"),
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		// Test that we can assign from a constant without type errors
		_ := bar.Kind
		return nil
	})
}
