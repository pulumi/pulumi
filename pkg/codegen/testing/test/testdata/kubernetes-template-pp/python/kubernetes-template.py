import pulumi
import pulumi_kubernetes as kubernetes

argocd_server_deployment = kubernetes.apps.v1.Deployment("argocd_serverDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata={
        "name": "argocd-server",
    },
    spec={
        "selector": kubernetes.meta.v1.LabelSelectorArgs(
            match_labels={
                "app": "server",
            },
        ),
        "replicas": 1,
        "template": kubernetes.core.v1.PodTemplateSpecArgs(
            metadata={
                "labels": {
                    "app": "server",
                },
            },
            spec={
                "containers": [kubernetes.core.v1.ContainerArgs(
                    name="nginx",
                    image="nginx",
                    readiness_probe={
                        "httpGet": {
                            "port": 8080,
                        },
                    },
                )],
            },
        ),
    })
