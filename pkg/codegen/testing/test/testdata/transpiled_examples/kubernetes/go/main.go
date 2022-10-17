package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		hostname := "example.com"
		if param := cfg.Get("hostname"); param != "" {
			hostname = param
		}
		nginxdemo, err := corev1.NewNamespace(ctx, "nginxdemo", nil)
		if err != nil {
			return err
		}
		_, err = appsv1.NewDeployment(ctx, "app", &appsv1.DeploymentArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Namespace: nginxdemo.Metadata.ApplyT(func(metadata metav1.ObjectMeta) (string, error) {
					return metadata.Name, nil
				}).(pulumi.StringOutput),
			},
			Spec: &appsv1.DeploymentSpecArgs{
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app.kubernetes.io/name": pulumi.String("nginx-demo"),
					},
				},
				Replicas: pulumi.Int(1),
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: pulumi.StringMap{
							"app.kubernetes.io/name": pulumi.String("nginx-demo"),
						},
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String("app"),
								Image: pulumi.String("nginx:1.15-alpine"),
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		service, err := corev1.NewService(ctx, "service", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Namespace: nginxdemo.Metadata.ApplyT(func(metadata metav1.ObjectMeta) (string, error) {
					return metadata.Name, nil
				}).(pulumi.StringOutput),
				Labels: pulumi.StringMap{
					"app.kubernetes.io/name": pulumi.String("nginx-demo"),
				},
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("ClusterIP"),
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(80),
						TargetPort: pulumi.Any(80),
						Protocol:   pulumi.String("TCP"),
					},
				},
				Selector: pulumi.StringMap{
					"app.kubernetes.io/name": pulumi.String("nginx-demo"),
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = networkingv1.NewIngress(ctx, "ingress", &networkingv1.IngressArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Namespace: nginxdemo.Metadata.ApplyT(func(metadata metav1.ObjectMeta) (string, error) {
					return metadata.Name, nil
				}).(pulumi.StringOutput),
			},
			Spec: &networkingv1.IngressSpecArgs{
				Rules: networkingv1.IngressRuleArray{
					&networkingv1.IngressRuleArgs{
						Host: pulumi.String(hostname),
						Http: &networkingv1.HTTPIngressRuleValueArgs{
							Paths: networkingv1.HTTPIngressPathArray{
								&networkingv1.HTTPIngressPathArgs{
									Path:     pulumi.String("/"),
									PathType: pulumi.String("Prefix"),
									Backend: &networkingv1.IngressBackendArgs{
										Service: &networkingv1.IngressServiceBackendArgs{
											Name: service.Metadata.ApplyT(func(metadata metav1.ObjectMeta) (string, error) {
												return metadata.Name, nil
											}).(pulumi.StringOutput),
											Port: &networkingv1.ServiceBackendPortArgs{
												Number: pulumi.Int(80),
											},
										},
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
