import pulumi
import pulumi_kubernetes as kubernetes

bar = kubernetes.core.v1.Pod("bar",
    api_version="v1",
    metadata=kubernetes.meta.v1.ObjectMetaArgs(
        namespace="foo",
        name="bar",
        labels={
            "app.kubernetes.io/name": "cilium-agent",
            "app.kubernetes.io/part-of": "cilium",
            "k8s-app": "cilium",
        },
    ),
    spec=kubernetes.core.v1.PodSpecArgs(
        containers=[
            kubernetes.core.v1.ContainerArgs(
                name="nginx",
                image="nginx:1.14-alpine",
                ports=[kubernetes.core.v1.ContainerPortArgs(
                    container_port=80,
                )],
                resources=kubernetes.core.v1.ResourceRequirementsArgs(
                    limits={
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                ),
            ),
            kubernetes.core.v1.ContainerArgs(
                name="nginx2",
                image="nginx:1.14-alpine",
                ports=[kubernetes.core.v1.ContainerPortArgs(
                    container_port=80,
                )],
                resources=kubernetes.core.v1.ResourceRequirementsArgs(
                    limits={
                        "memory": "20Mi",
                        "cpu": "0.2",
                    },
                ),
            ),
        ],
    ))
# Test that we can assign from a constant without type errors
kind = bar.kind
