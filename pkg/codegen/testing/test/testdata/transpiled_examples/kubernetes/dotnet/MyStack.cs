using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

class MyStack : Stack
{
    public MyStack()
    {
        var config = new Config();
        var hostname = config.Get("hostname") ?? "example.com";
        var nginxDemo = new Kubernetes.Core.V1.Namespace("nginx-demo", new Kubernetes.Types.Inputs.Core.V1.NamespaceArgs
        {
        });
        var app = new Kubernetes.Apps.V1.Deployment("app", new Kubernetes.Types.Inputs.Apps.V1.DeploymentArgs
        {
            Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
            {
                Namespace = nginxDemo.Metadata.Apply(metadata => metadata?.Name),
            },
            Spec = new Kubernetes.Types.Inputs.Apps.V1.DeploymentSpecArgs
            {
                Selector = new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
                {
                    MatchLabels = 
                    {
                        { "app.kubernetes.io/name", "nginx-demo" },
                    },
                },
                Replicas = 1,
                Template = new Kubernetes.Types.Inputs.Core.V1.PodTemplateSpecArgs
                {
                    Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
                    {
                        Labels = 
                        {
                            { "app.kubernetes.io/name", "nginx-demo" },
                        },
                    },
                    Spec = new Kubernetes.Types.Inputs.Core.V1.PodSpecArgs
                    {
                        Containers = 
                        {
                            new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
                            {
                                Name = "app",
                                Image = "nginx:1.15-alpine",
                            },
                        },
                    },
                },
            },
        });
        var service = new Kubernetes.Core.V1.Service("service", new Kubernetes.Types.Inputs.Core.V1.ServiceArgs
        {
            Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
            {
                Namespace = nginxDemo.Metadata.Apply(metadata => metadata?.Name),
                Labels = 
                {
                    { "app.kubernetes.io/name", "nginx-demo" },
                },
            },
            Spec = new Kubernetes.Types.Inputs.Core.V1.ServiceSpecArgs
            {
                Type = "ClusterIP",
                Ports = 
                {
                    new Kubernetes.Types.Inputs.Core.V1.ServicePortArgs
                    {
                        Port = 80,
                        TargetPort = 80,
                        Protocol = "TCP",
                    },
                },
                Selector = 
                {
                    { "app.kubernetes.io/name", "nginx-demo" },
                },
            },
        });
        var ingress = new Kubernetes.Networking.V1.Ingress("ingress", new Kubernetes.Types.Inputs.Networking.V1.IngressArgs
        {
            Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
            {
                Namespace = nginxDemo.Metadata.Apply(metadata => metadata?.Name),
            },
            Spec = new Kubernetes.Types.Inputs.Networking.V1.IngressSpecArgs
            {
                Rules = 
                {
                    new Kubernetes.Types.Inputs.Networking.V1.IngressRuleArgs
                    {
                        Host = hostname,
                        Http = new Kubernetes.Types.Inputs.Networking.V1.HTTPIngressRuleValueArgs
                        {
                            Paths = 
                            {
                                new Kubernetes.Types.Inputs.Networking.V1.HTTPIngressPathArgs
                                {
                                    Path = "/",
                                    PathType = "Prefix",
                                    Backend = new Kubernetes.Types.Inputs.Networking.V1.IngressBackendArgs
                                    {
                                        Service = new Kubernetes.Types.Inputs.Networking.V1.IngressServiceBackendArgs
                                        {
                                            Name = service.Metadata.Apply(metadata => metadata?.Name),
                                            Port = new Kubernetes.Types.Inputs.Networking.V1.ServiceBackendPortArgs
                                            {
                                                Number = 80,
                                            },
                                        },
                                    },
                                },
                            },
                        },
                    },
                },
            },
        });
    }

}
