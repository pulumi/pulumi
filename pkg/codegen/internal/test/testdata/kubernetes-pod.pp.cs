using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

class MyStack : Stack
{
    public MyStack()
    {
        var bar = new Kubernetes.Core.v1.Pod("bar", new Kubernetes.Core.v1.PodArgs
        {
            ApiVersion = "v1",
            Kind = "Pod",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Namespace = "foo",
                Name = "bar",
            },
            Spec = new Kubernetes.Core.Inputs.PodSpecArgs
            {
                Containers = 
                {
                    new Kubernetes.Core.Inputs.ContainerArgs
                    {
                        Name = "nginx",
                        Image = "nginx:1.14-alpine",
                        Resources = new Kubernetes.Core.Inputs.ResourceRequirementsArgs
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
    }

}
