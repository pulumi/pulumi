using System.Collections.Generic;
using Pulumi;
using Kubernetes = Pulumi.Kubernetes;

return await Deployment.RunAsync(() => 
{
    var argocd_serverDeployment = new Kubernetes.Apps.V1.Deployment("argocd_serverDeployment", new()
    {
        ApiVersion = "apps/v1",
        Kind = "Deployment",
        Metadata = 
        {
            { "name", "argocd-server" },
        },
        Spec = 
        {
            { "selector", new Kubernetes.Types.Inputs.Meta.V1.LabelSelectorArgs
            {
                MatchLabels = 
                {
                    { "app", "server" },
                },
            } },
            { "replicas", 1 },
            { "template", new Kubernetes.Types.Inputs.Core.V1.PodTemplateSpecArgs
            {
                Metadata = 
                {
                    { "labels", 
                    {
                        { "app", "server" },
                    } },
                },
                Spec = 
                {
                    { "containers", new[]
                    {
                        new Kubernetes.Types.Inputs.Core.V1.ContainerArgs
                        {
                            Name = "nginx",
                            Image = "nginx",
                            ReadinessProbe = 
                            {
                                { "httpGet", 
                                {
                                    { "port", 8080 },
                                } },
                            },
                        },
                    } },
                },
            } },
        },
    });

});

