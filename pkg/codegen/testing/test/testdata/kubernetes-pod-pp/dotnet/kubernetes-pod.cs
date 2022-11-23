using System.Collections.Generic;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var bar = new Kubernetes.Core.V1.Pod("bar", new()
    {
        ApiVersion = "v1",
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
                    Ports = new[]
                    {
                        new Kubernetes.Types.Inputs.Core.V1.ContainerPortArgs
                        {
                            ContainerPortValue = 80,
                        },
                    },
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

    var kind = bar.Kind;

});

