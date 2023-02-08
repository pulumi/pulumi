using System.Collections.Generic;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var bar = new Kubernetes.Core.V1.Pod("bar", new()
    {
        ApiVersion = "v1",
        Metadata = 
        {
            { "namespace", "foo" },
            { "name", "bar" },
        },
        Spec = 
        {
            { "containers", new[]
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
                    Resources = 
                    {
                        { "limits", 
                        {
                            { "memory", "20Mi" },
                            { "cpu", "0.2" },
                        } },
                    },
                },
                new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
                {
                    Name = "nginx2",
                    Image = "nginx:1.14-alpine",
                    Ports = new[]
                    {
                        new Kubernetes.Types.Inputs.Core.V1.ContainerPortArgs
                        {
                            ContainerPortValue = 80,
                        },
                    },
                    Resources = 
                    {
                        { "limits", 
                        {
                            { "memory", "20Mi" },
                            { "cpu", "0.2" },
                        } },
                    },
                },
            } },
        },
    });

    var kind = bar.Kind;

});

