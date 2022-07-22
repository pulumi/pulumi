using System.Collections.Generic;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var bar = new Kubernetes.Core.V1.Pod("bar", new()
    {
        ApiVersion = "v1",
        Kind = "Pod",
        Metadata = new Kubernetes.Types.Inputs.Meta.V1.ObjectMetaArgs
        {
            Namespace = "foo",
            Name = "bar",
        },
        Spec = new Kubernetes.Types.Inputs.Core.V1.PodSpecArgs
        {
            Containers = new[]
            {
                new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
                {
                    Name = "nginx",
                    Image = "nginx:1.14-alpine",
                    Resources = new Kubernetes.Types.Inputs.Core.V1.ResourceRequirementsArgs
                    {
                        Limits = 
                        {
                            { "memory", "20Mi" },
                            { "cpu", "0.2" },
                        },
                    },
                },
            },
        },
    });

});

