import pulumi
import pulumi_kubernetes as kubernetes

argocd_server_deployment = kubernetes.apps.v1.Deployment("argocd_serverDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata={
        "name": "argocd-server",
    },
    spec={
        "selector": {
            "match_labels": {
                "app": "server",
            },
        },
        "replicas": 1,
        "template": {
            "metadata": {
                "labels": {
                    "app": "server",
                },
            },
            "spec": {
                "containers": [{
                    "name": "nginx",
                    "image": "nginx",
                    "readiness_probe": {
                        "http_get": {
                            "port": 8080,
                        },
                    },
                }],
            },
        },
    })
