import pulumi
import pulumi_kubernetes as kubernetes

argocd_server_deployment = kubernetes.apps.v1.Deployment("argocd_serverDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata=kubernetes.meta.v1.ObjectMetaArrgs(
        name="argocd-server",
    ),
    spec=kubernetes.apps.v1.DeploymentSpecArrgs(
        selector=kubernetes.meta.v1.LabelSelectorArrgs(
            match_labels={
                "app": "server",
            },
        ),
        replicas=1,
        template=kubernetes.core.v1.PodTemplateSpecArrgs(
            metadata=kubernetes.meta.v1.ObjectMetaArrgs(
                labels={
                    "app": "server",
                },
            ),
            spec=kubernetes.core.v1.PodSpecArrgs(
                containers=[kubernetes.core.v1.ContainerArrgs(
                    name="nginx",
                    image="nginx",
                    readiness_probe=kubernetes.core.v1.ProbeArrgs(
                        http_get=kubernetes.core.v1.HTTPGetActionArrgs(
                            port=8080,
                        ),
                    ),
                )],
            ),
        ),
    ))
