import pulumi
import pulumi_kubernetes as kubernetes

bar = kubernetes.core.v1.Pod("bar",
    api_version="v1",
    metadata={
        "namespace": "foo",
        "name": "bar",
        "labels": {
            "app_kubernetes_io_name": "cilium-agent",
            "app_kubernetes_io_part_of": "cilium",
            "k8s_app": "cilium",
        },
    },
    spec={
        "containers": [
            {
                "name": "nginx",
                "image": "nginx:1.14-alpine",
                "ports": [{
                    "container_port": 80,
                }],
                "resources": {
                    "limits": {
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                },
            },
            {
                "name": "nginx2",
                "image": "nginx:1.14-alpine",
                "ports": [{
                    "container_port": 80,
                }],
                "resources": {
                    "limits": {
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                },
            },
        ],
    })
# Test that we can assign from a constant without type errors
kind = bar.kind
