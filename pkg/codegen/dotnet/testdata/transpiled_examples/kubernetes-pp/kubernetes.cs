using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var hostname = config.Get("hostname") ?? "example.com";
    var nginxDemo = new Kubernetes.Core.V1.Namespace("nginx-demo");

    var app = new Kubernetes.Apps.V1.Deployment("app", new()
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
                    Containers = new[]
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

    var service = new Kubernetes.Core.V1.Service("service", new()
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
            Type = Kubernetes.Core.V1.ServiceSpecType.ClusterIP,
            Ports = new[]
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

    var ingress = new Kubernetes.Networking.V1.Ingress("ingress", new()
    {
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            Namespace = nginxDemo.Metadata.Apply(metadata => metadata?.Name),
        },
        Spec = new Kubernetes.Types.Inputs.Networking.V1.IngressSpecArgs
        {
            Rules = new[]
            {
                new Kubernetes.Types.Inputs.Networking.V1.IngressRuleArgs
                {
                    Host = hostname,
                    Http = new Kubernetes.Types.Inputs.Networking.V1.HTTPIngressRuleValueArgs
                    {
                        Paths = new[]
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

});

