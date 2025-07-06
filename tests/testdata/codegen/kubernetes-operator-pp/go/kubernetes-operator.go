package main

import (
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/meta/v1"
	rbacv1 "github.com/pulumi/pulumi-kubernetes/sdk/v3/go/kubernetes/rbac/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := appsv1.NewDeployment(ctx, "pulumi_kubernetes_operatorDeployment", &appsv1.DeploymentArgs{
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			Metadata: &*metav1.ObjectMetaArgs{
				Name: "pulumi-kubernetes-operator",
			},
			Spec: &*appsv1.DeploymentSpecArgs{
				Replicas: 1,
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: map[string]pulumi.String{
						"name": pulumi.String("pulumi-kubernetes-operator"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &*metav1.ObjectMetaArgs{
						Labels: map[string]pulumi.String{
							"name": pulumi.String("pulumi-kubernetes-operator"),
						},
					},
					Spec: &*corev1.PodSpecArgs{
						ServiceAccountName: "pulumi-kubernetes-operator",
						ImagePullSecrets: []corev1.LocalObjectReferenceArgs{
							{
								Name: "pulumi-kubernetes-operator",
							},
						},
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String("pulumi-kubernetes-operator"),
								Image: "pulumi/pulumi-kubernetes-operator:v0.0.2",
								Command: []pulumi.String{
									pulumi.String("pulumi-kubernetes-operator"),
								},
								Args: []pulumi.String{
									pulumi.String("--zap-level=debug"),
								},
								ImagePullPolicy: "Always",
								Env: []corev1.EnvVarArgs{
									{
										Name: pulumi.String("WATCH_NAMESPACE"),
										ValueFrom: {
											FieldRef: {
												FieldPath: pulumi.String("metadata.namespace"),
											},
										},
									},
									{
										Name: pulumi.String("POD_NAME"),
										ValueFrom: {
											FieldRef: {
												FieldPath: pulumi.String("metadata.name"),
											},
										},
									},
									{
										Name:  pulumi.String("OPERATOR_NAME"),
										Value: "pulumi-kubernetes-operator",
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
		_, err = rbacv1.NewRole(ctx, "pulumi_kubernetes_operatorRole", &rbacv1.RoleArgs{
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
			Metadata: &*metav1.ObjectMetaArgs{
				CreationTimestamp: nil,
				Name:              "pulumi-kubernetes-operator",
			},
			Rules: []rbacv1.PolicyRuleArgs{
				{
					ApiGroups: []pulumi.String{
						pulumi.String(""),
					},
					Resources: []pulumi.String{
						pulumi.String("pods"),
						pulumi.String("services"),
						pulumi.String("services/finalizers"),
						pulumi.String("endpoints"),
						pulumi.String("persistentvolumeclaims"),
						pulumi.String("events"),
						pulumi.String("configmaps"),
						pulumi.String("secrets"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String("apps"),
					},
					Resources: []pulumi.String{
						pulumi.String("deployments"),
						pulumi.String("daemonsets"),
						pulumi.String("replicasets"),
						pulumi.String("statefulsets"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String("monitoring.coreos.com"),
					},
					Resources: []pulumi.String{
						pulumi.String("servicemonitors"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
						pulumi.String("create"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String("apps"),
					},
					ResourceNames: []pulumi.String{
						pulumi.String("pulumi-kubernetes-operator"),
					},
					Resources: []pulumi.String{
						pulumi.String("deployments/finalizers"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("update"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String(""),
					},
					Resources: []pulumi.String{
						pulumi.String("pods"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String("apps"),
					},
					Resources: []pulumi.String{
						pulumi.String("replicasets"),
						pulumi.String("deployments"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("get"),
					},
				},
				{
					ApiGroups: []pulumi.String{
						pulumi.String("pulumi.com"),
					},
					Resources: []pulumi.String{
						pulumi.String("*"),
					},
					Verbs: pulumi.StringArray{
						pulumi.String("create"),
						pulumi.String("delete"),
						pulumi.String("get"),
						pulumi.String("list"),
						pulumi.String("patch"),
						pulumi.String("update"),
						pulumi.String("watch"),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = rbacv1.NewRoleBinding(ctx, "pulumi_kubernetes_operatorRoleBinding", &rbacv1.RoleBindingArgs{
			Kind:       "RoleBinding",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Metadata: &*metav1.ObjectMetaArgs{
				Name: "pulumi-kubernetes-operator",
			},
			Subjects: []rbacv1.SubjectArgs{
				{
					Kind: pulumi.String("ServiceAccount"),
					Name: pulumi.String("pulumi-kubernetes-operator"),
				},
			},
			RoleRef: &rbacv1.RoleRefArgs{
				Kind:     pulumi.String("Role"),
				Name:     pulumi.String("pulumi-kubernetes-operator"),
				ApiGroup: pulumi.String("rbac.authorization.k8s.io"),
			},
		})
		if err != nil {
			return err
		}
		_, err = corev1.NewServiceAccount(ctx, "pulumi_kubernetes_operatorServiceAccount", &corev1.ServiceAccountArgs{
			ApiVersion: "v1",
			Kind:       "ServiceAccount",
			Metadata: &*metav1.ObjectMetaArgs{
				Name: "pulumi-kubernetes-operator",
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
