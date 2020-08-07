using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

class MyStack : Stack
{
    public MyStack()
    {
        var pulumi_kubernetes_operatorDeployment = new Kubernetes.Apps.v1.Deployment("pulumi_kubernetes_operatorDeployment", new Kubernetes.Apps.v1.DeploymentArgs
        {
            ApiVersion = "apps/v1",
            Kind = "Deployment",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Spec = new Kubernetes.Apps.Inputs.DeploymentSpecArgs
            {
                Replicas = 1,
                Selector = new Kubernetes.Meta.Inputs.LabelSelectorArgs
                {
                    MatchLabels = 
                    {
                        { "name", "pulumi-kubernetes-operator" },
                    },
                },
                Template = new Kubernetes.Core.Inputs.PodTemplateSpecArgs
                {
                    Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
                    {
                        Labels = 
                        {
                            { "name", "pulumi-kubernetes-operator" },
                        },
                    },
                    Spec = new Kubernetes.Core.Inputs.PodSpecArgs
                    {
                        ServiceAccountName = "pulumi-kubernetes-operator",
                        ImagePullSecrets = 
                        {
                            new Kubernetes.Core.Inputs.LocalObjectReferenceArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                            },
                        },
                        Containers = 
                        {
                            new Kubernetes.Core.Inputs.ContainerArgs
                            {
                                Name = "pulumi-kubernetes-operator",
                                Image = "pulumi/pulumi-kubernetes-operator:v0.0.2",
                                Command = 
                                {
                                    "pulumi-kubernetes-operator",
                                },
                                Args = 
                                {
                                    "--zap-level=debug",
                                },
                                ImagePullPolicy = "Always",
                                Env = 
                                {
                                    new Kubernetes.Core.Inputs.EnvVarArgs
                                    {
                                        Name = "WATCH_NAMESPACE",
                                        ValueFrom = new Kubernetes.Core.Inputs.EnvVarSourceArgs
                                        {
                                            FieldRef = new Kubernetes.Core.Inputs.ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.namespace",
                                            },
                                        },
                                    },
                                    new Kubernetes.Core.Inputs.EnvVarArgs
                                    {
                                        Name = "POD_NAME",
                                        ValueFrom = new Kubernetes.Core.Inputs.EnvVarSourceArgs
                                        {
                                            FieldRef = new Kubernetes.Core.Inputs.ObjectFieldSelectorArgs
                                            {
                                                FieldPath = "metadata.name",
                                            },
                                        },
                                    },
                                    new Kubernetes.Core.Inputs.EnvVarArgs
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
        var pulumi_kubernetes_operatorRole = new Kubernetes.Rbac.authorization.k8s.io.v1.Role("pulumi_kubernetes_operatorRole", new Kubernetes.Rbac.authorization.k8s.io.v1.RoleArgs
        {
            ApiVersion = "rbac.authorization.k8s.io/v1",
            Kind = "Role",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                CreationTimestamp = null,
                Name = "pulumi-kubernetes-operator",
            },
            Rules = 
            {
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "",
                    },
                    Resources = 
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
                    Verbs = 
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
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    Resources = 
                    {
                        "deployments",
                        "daemonsets",
                        "replicasets",
                        "statefulsets",
                    },
                    Verbs = 
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
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "monitoring.coreos.com",
                    },
                    Resources = 
                    {
                        "servicemonitors",
                    },
                    Verbs = 
                    {
                        "get",
                        "create",
                    },
                },
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    ResourceNames = 
                    {
                        "pulumi-kubernetes-operator",
                    },
                    Resources = 
                    {
                        "deployments/finalizers",
                    },
                    Verbs = 
                    {
                        "update",
                    },
                },
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "",
                    },
                    Resources = 
                    {
                        "pods",
                    },
                    Verbs = 
                    {
                        "get",
                    },
                },
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "apps",
                    },
                    Resources = 
                    {
                        "replicasets",
                        "deployments",
                    },
                    Verbs = 
                    {
                        "get",
                    },
                },
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.PolicyRuleArgs
                {
                    ApiGroups = 
                    {
                        "pulumi.com",
                    },
                    Resources = 
                    {
                        "*",
                    },
                    Verbs = 
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
        var pulumi_kubernetes_operatorRoleBinding = new Kubernetes.Rbac.authorization.k8s.io.v1.RoleBinding("pulumi_kubernetes_operatorRoleBinding", new Kubernetes.Rbac.authorization.k8s.io.v1.RoleBindingArgs
        {
            Kind = "RoleBinding",
            ApiVersion = "rbac.authorization.k8s.io/v1",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
            Subjects = 
            {
                new Kubernetes.Rbac.authorization.k8s.io.Inputs.SubjectArgs
                {
                    Kind = "ServiceAccount",
                    Name = "pulumi-kubernetes-operator",
                },
            },
            RoleRef = new Kubernetes.Rbac.authorization.k8s.io.Inputs.RoleRefArgs
            {
                Kind = "Role",
                Name = "pulumi-kubernetes-operator",
                ApiGroup = "rbac.authorization.k8s.io",
            },
        });
        var pulumi_kubernetes_operatorServiceAccount = new Kubernetes.Core.v1.ServiceAccount("pulumi_kubernetes_operatorServiceAccount", new Kubernetes.Core.v1.ServiceAccountArgs
        {
            ApiVersion = "v1",
            Kind = "ServiceAccount",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "pulumi-kubernetes-operator",
            },
        });
    }

}
