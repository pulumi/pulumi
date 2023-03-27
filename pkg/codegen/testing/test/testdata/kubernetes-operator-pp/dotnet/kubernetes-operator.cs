using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var pulumi_kubernetes_operatorDeployment = new Kubernetes.Apps.V1.Deployment("pulumi_kubernetes_operatorDeployment", new()
    {
        ApiVersion = "apps/v1",
        Kind = "Deployment",
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            Name = "pulumi-kubernetes-operator",
        },
        Spec = new Kubernetes.Types.Inputs.Apps.V1.DeploymentSpecArgs
        {
            Replicas = 1,
            Selector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
            {
                MatchLabels = 
                {
                    { "name", "pulumi-kubernetes-operator" },
                },
            },
            Template = new Kubernetes.Types.Inputs.Core.V1.PodTemplateSpecArgs
            {
                Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
                {
                    Labels = 
                    {
                        { "name", "pulumi-kubernetes-operator" },
                    },
                },
                Spec = new Kubernetes.Types.Inputs.Core.V1.PodSpecArgs
                {
                    ServiceAccountName = "pulumi-kubernetes-operator",
                    ImagePullSecrets = new[]
                    {
                        new Kubernetes.Types.Inputs.Core.V1.LocalObjectReferenceArgs
                        {
                            Name = "pulumi-kubernetes-operator",
                        },
                    },
                    Containers = new[]
                    {
                        new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
                        {
                            Name = "pulumi-kubernetes-operator",
                            Image = "pulumi/pulumi-kubernetes-operator:v0.0.2",
                            Command = new[]
                            {
                                "pulumi-kubernetes-operator",
                            },
                            Args = new[]
                            {
                                "--zap-level=debug",
                            },
                            ImagePullPolicy = "Always",
                            Env = new[]
                            {
                                new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
                                {
                                    Name = "WATCH_NAMESPACE",
                                    ValueFrom = new Kubernetes.Types.Inputs.Core.V1.EnvVarSourceArgs
                                    {
                                        FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                                        {
                                            FieldPath = "metadata.namespace",
                                        },
                                    },
                                },
                                new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
                                {
                                    Name = "POD_NAME",
                                    ValueFrom = new Kubernetes.Types.Inputs.Core.V1.EnvVarSourceArgs
                                    {
                                        FieldRef = new Kubernetes.Types.Inputs.Core.V1.ObjectFieldSelectorArgs
                                        {
                                            FieldPath = "metadata.name",
                                        },
                                    },
                                },
                                new Kubernetes.Types.Inputs.Core.V1.EnvVarArgs
                                {
                                    Name = "OPERATOR_NAME",
                                    Value = "pulumi-kubernetes-operator",
                                },
                            },
                        },
                    },
                },
            },
        },
    });

    var pulumi_kubernetes_operatorRole = new Kubernetes.Rbac.V1.Role("pulumi_kubernetes_operatorRole", new()
    {
        ApiVersion = "rbac.authorization.k8s.io/v1",
        Kind = "Role",
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            CreationTimestamp = null,
            Name = "pulumi-kubernetes-operator",
        },
        Rules = new[]
        {
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "",
                },
                Resources = new[]
                {
                    "pods",
                    "services",
                    "services/finalizers",
                    "endpoints",
                    "persistentvolumeclaims",
                    "events",
                    "configmaps",
                    "secrets",
                },
                Verbs = new[]
                {
                    "create",
                    "delete",
                    "get",
                    "list",
                    "patch",
                    "update",
                    "watch",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "apps",
                },
                Resources = new[]
                {
                    "deployments",
                    "daemonsets",
                    "replicasets",
                    "statefulsets",
                },
                Verbs = new[]
                {
                    "create",
                    "delete",
                    "get",
                    "list",
                    "patch",
                    "update",
                    "watch",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "monitoring.coreos.com",
                },
                Resources = new[]
                {
                    "servicemonitors",
                },
                Verbs = new[]
                {
                    "get",
                    "create",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "apps",
                },
                ResourceNames = new[]
                {
                    "pulumi-kubernetes-operator",
                },
                Resources = new[]
                {
                    "deployments/finalizers",
                },
                Verbs = new[]
                {
                    "update",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "",
                },
                Resources = new[]
                {
                    "pods",
                },
                Verbs = new[]
                {
                    "get",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "apps",
                },
                Resources = new[]
                {
                    "replicasets",
                    "deployments",
                },
                Verbs = new[]
                {
                    "get",
                },
            },
            new Kubernetes.Types.Inputs.Rbac.V1.PolicyRuleArgs
            {
                ApiGroups = new[]
                {
                    "pulumi.com",
                },
                Resources = new[]
                {
                    "*",
                },
                Verbs = new[]
                {
                    "create",
                    "delete",
                    "get",
                    "list",
                    "patch",
                    "update",
                    "watch",
                },
            },
        },
    });

    var pulumi_kubernetes_operatorRoleBinding = new Kubernetes.Rbac.V1.RoleBinding("pulumi_kubernetes_operatorRoleBinding", new()
    {
        Kind = "RoleBinding",
        ApiVersion = "rbac.authorization.k8s.io/v1",
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            Name = "pulumi-kubernetes-operator",
        },
        Subjects = new[]
        {
            new Kubernetes.Types.Inputs.Rbac.V1.SubjectArgs
            {
                Kind = "ServiceAccount",
                Name = "pulumi-kubernetes-operator",
            },
        },
        RoleRef = new Kubernetes.Types.Inputs.Rbac.V1.RoleRefArgs
        {
            Kind = "Role",
            Name = "pulumi-kubernetes-operator",
            ApiGroup = "rbac.authorization.k8s.io",
        },
    });

    var pulumi_kubernetes_operatorServiceAccount = new Kubernetes.Core.V1.ServiceAccount("pulumi_kubernetes_operatorServiceAccount", new()
    {
        ApiVersion = "v1",
        Kind = "ServiceAccount",
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            Name = "pulumi-kubernetes-operator",
        },
    });

});

