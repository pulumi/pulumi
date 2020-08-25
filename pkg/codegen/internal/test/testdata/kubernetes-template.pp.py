import pulumi
import pulumi_kubernetes as kubernetes

argocd_server_deployment = kubernetes.apps.v1.Deployment("argocd_serverDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata={
        "name": "argocd-server",
    },
    spec={
        "template": {
            "spec": {
                "containers": [{
                    "readiness_probe": {
                        "http_get": {
                            "port": 8080,
                        },
                    },
                }],
            },
        },
    })
