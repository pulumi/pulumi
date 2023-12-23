package main

import (
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		bar, err := corev1.NewPod(ctx, "bar", &corev1.PodArgs{
			ApiVersion: pulumi.String("v1"),
			Metadata: &metav1.ObjectMetaArgs{
				Namespace: pulumi.String("foo"),
				Name:      pulumi.String("bar"),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name":    pulumi.String("cilium-agent"),
					"app.kubernetes.io/part-of": pulumi.String("cilium"),
					"k8s-app":                   pulumi.String("cilium"),
				},
			},
			Spec: &corev1.PodSpecArgs{
				Containers: []corev1.ContainerArgs{
					{
						Name:  pulumi.String("nginx"),
						Image: pulumi.String("nginx:1.14-alpine"),
						Ports: corev1.ContainerPortArray{
							{
								ContainerPort: pulumi.Int(80),
							},
						},
						Resources: {
							Limits: {
								"memory": pulumi.String("20Mi"),
								"cpu":    pulumi.String("0.2"),
							},
						},
					},
					{
						Name:  pulumi.String("nginx2"),
						Image: pulumi.String("nginx:1.14-alpine"),
						Ports: corev1.ContainerPortArray{
							{
								ContainerPort: pulumi.Int(80),
							},
						},
						Resources: {
							Limits: {
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
		_ := bar.Kind
		return nil
	})
}
