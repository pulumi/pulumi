import pulumi
import pulumi_kubernetes as kubernetes

argocd_server_deployment = kubernetes.apps.v1.Deployment("argocd_serverDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata=kubernetes.meta.v1.ObjectMetaArgs(
        name="argocd-server",
    ),
    spec=kubernetes.apps.v1.DeploymentSpecArgs(
        selector=kubernetes.meta.v1.LabelSelectorArgs(
            match_labels={
                "app": "server",
            },
        ),
        replicas=1,
        template=kubernetes.core.v1.PodTemplateSpecArgs(
            metadata=kubernetes.meta.v1.ObjectMetaArgs(
                labels={
                    "app": "server",
                },
            ),
            spec=kubernetes.core.v1.PodSpecArgs(
                containers=[kubernetes.core.v1.ContainerArgs(
                    name="nginx",
                    image="nginx",
                    readiness_probe=kubernetes.core.v1.ProbeArgs(
                        http_get=kubernetes.core.v1.HTTPGetActionArgs(
                            port=8080,
                        ),
                    ),
                )],
            ),
        ),
    ))
