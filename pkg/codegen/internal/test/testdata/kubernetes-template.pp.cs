using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

class MyStack : Stack
{
    public MyStack()
    {
        var argocd_serverDeployment = new Kubernetes.Apps.v1.Deployment("argocd_serverDeployment", new Kubernetes.Apps.v1.DeploymentArgs
        {
            ApiVersion = "apps/v1",
            Kind = "Deployment",
            Metadata = new Kubernetes.Meta.Inputs.ObjectMetaArgs
            {
                Name = "argocd-server",
            },
            Spec = new Kubernetes.Apps.Inputs.DeploymentSpecArgs
            {
                Template = new Kubernetes.Core.Inputs.PodTemplateSpecArgs
                {
                    Spec = new Kubernetes.Core.Inputs.PodSpecArgs
                    {
                        Containers = 
                        {
                            new Kubernetes.Core.Inputs.ContainerArgs
                            {
                                ReadinessProbe = new Kubernetes.Core.Inputs.ProbeArgs
                                {
                                    HttpGet = new Kubernetes.Core.Inputs.HTTPGetActionArgs
                                    {
                                        Port = 8080,
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
