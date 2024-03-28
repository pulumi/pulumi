import pulumi
import pulumi_kubernetes as kubernetes

config = pulumi.Config()
hostname = config.get("hostname")
if hostname is None:
    hostname = "example.com"
nginx_demo = kubernetes.core.v1.Namespace("nginx-demo")
app = kubernetes.apps.v1.Deployment("app",
    metadata=kubernetes.meta.v1.ObjectMetaArgs(
        namespace=nginx_demo.metadata.name,
    ),
    spec=kubernetes.apps.v1.DeploymentSpecArgs(
        selector=kubernetes.meta.v1.LabelSelectorArgs(
            match_labels={
                "app.kubernetes.io/name": "nginx-demo",
            },
        ),
        replicas=1,
        template=kubernetes.core.v1.PodTemplateSpecArgs(
            metadata=kubernetes.meta.v1.ObjectMetaArgs(
                labels={
                    "app.kubernetes.io/name": "nginx-demo",
                },
            ),
            spec=kubernetes.core.v1.PodSpecArgs(
                containers=[kubernetes.core.v1.ContainerArgs(
                    name="app",
                    image="nginx:1.15-alpine",
                )],
            ),
        ),
    ))
service = kubernetes.core.v1.Service("service",
    metadata=kubernetes.meta.v1.ObjectMetaArgs(
        namespace=nginx_demo.metadata.name,
        labels={
            "app.kubernetes.io/name": "nginx-demo",
        },
    ),
    spec=kubernetes.core.v1.ServiceSpecArgs(
        type=kubernetes.core.v1.ServiceSpecType.CLUSTER_IP,
        ports=[kubernetes.core.v1.ServicePortArgs(
            port=80,
            target_port=80,
            protocol="TCP",
        )],
        selector={
            "app.kubernetes.io/name": "nginx-demo",
        },
    ))
ingress = kubernetes.networking.v1.Ingress("ingress",
    metadata=kubernetes.meta.v1.ObjectMetaArgs(
        namespace=nginx_demo.metadata.name,
    ),
    spec=kubernetes.networking.v1.IngressSpecArgs(
        rules=[kubernetes.networking.v1.IngressRuleArgs(
            host=hostname,
            http=kubernetes.networking.v1.HTTPIngressRuleValueArgs(
                paths=[kubernetes.networking.v1.HTTPIngressPathArgs(
                    path="/",
                    path_type="Prefix",
                    backend=kubernetes.networking.v1.IngressBackendArgs(
                        service=kubernetes.networking.v1.IngressServiceBackendArgs(
                            name=service.metadata.name,
                            port=kubernetes.networking.v1.ServiceBackendPortArgs(
                                number=80,
                            ),
                        ),
                    ),
                )],
            ),
        )],
    ))
