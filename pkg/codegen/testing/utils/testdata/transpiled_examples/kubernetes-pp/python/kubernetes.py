import pulumi
import pulumi_kubernetes as kubernetes

config = pulumi.Config()
hostname = config.get("hostname")
if hostname is None:
    hostname = "example.com"
nginx_demo = kubernetes.core.v1.Namespace("nginx-demo")
app = kubernetes.apps.v1.Deployment("app",
    metadata={
        "namespace": nginx_demo.metadata.name,
    },
    spec={
        "selector": {
            "match_labels": {
                "app.kubernetes.io/name": "nginx-demo",
            },
        },
        "replicas": 1,
        "template": {
            "metadata": {
                "labels": {
                    "app.kubernetes.io/name": "nginx-demo",
                },
            },
            "spec": {
                "containers": [{
                    "name": "app",
                    "image": "nginx:1.15-alpine",
                }],
            },
        },
    })
service = kubernetes.core.v1.Service("service",
    metadata={
        "namespace": nginx_demo.metadata.name,
        "labels": {
            "app.kubernetes.io/name": "nginx-demo",
        },
    },
    spec={
        "type": kubernetes.core.v1.ServiceSpecType.CLUSTER_IP,
        "ports": [{
            "port": 80,
            "target_port": 80,
            "protocol": "TCP",
        }],
        "selector": {
            "app.kubernetes.io/name": "nginx-demo",
        },
    })
ingress = kubernetes.networking.v1.Ingress("ingress",
    metadata={
        "namespace": nginx_demo.metadata.name,
    },
    spec={
        "rules": [{
            "host": hostname,
            "http": {
                "paths": [{
                    "path": "/",
                    "path_type": "Prefix",
                    "backend": {
                        "service": {
                            "name": service.metadata.name,
                            "port": {
                                "number": 80,
                            },
                        },
                    },
                }],
            },
        }],
    })
