import pulumi
import pulumi_kubernetes as kubernetes

bar = kubernetes.core.v1.Pod("bar",
    api_version="v1",
    metadata={
        "namespace": "foo",
        "name": "bar",
    },
    spec={
        "containers": [
            kubernetes.core.v1.ContainerArgs(
                name="nginx",
                image="nginx:1.14-alpine",
                ports=[kubernetes.core.v1.ContainerPortArgs(
                    container_port=80,
                )],
                resources={
                    "limits": {
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                },
            ),
            kubernetes.core.v1.ContainerArgs(
                name="nginx2",
                image="nginx:1.14-alpine",
                ports=[kubernetes.core.v1.ContainerPortArgs(
                    container_port=80,
                )],
                resources={
                    "limits": {
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                },
            ),
        ],
    })
kind = bar.kind
