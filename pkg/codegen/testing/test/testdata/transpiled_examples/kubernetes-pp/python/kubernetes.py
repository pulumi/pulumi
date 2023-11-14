import pulumi
import pulumi_kubernetes as kubernetes

config = pulumi.Config()
hostname = config.get("hostname")
if hostname is None:
    hostname = "example.com"
nginx_demo = kubernetes.core.v1.Namespace("nginx-demo")
app = kubernetes.apps.v1.Deployment("app",
    metadata=kubernetes.meta.v1.ObjectMetaArrgs(
        namespace=nginx_demo.metadata.name,
    ),
    spec=kubernetes.apps.v1.DeploymentSpecArrgs(
        selector=kubernetes.meta.v1.LabelSelectorArrgs(
            match_labels={
                "app.kubernetes.io/name": "nginx-demo",
            },
        ),
        replicas=1,
        template=kubernetes.core.v1.PodTemplateSpecArrgs(
            metadata=kubernetes.meta.v1.ObjectMetaArrgs(
                labels={
                    "app.kubernetes.io/name": "nginx-demo",
                },
            ),
            spec=kubernetes.core.v1.PodSpecArrgs(
                containers=[kubernetes.core.v1.ContainerArrgs(
                    name="app",
                    image="nginx:1.15-alpine",
                )],
            ),
        ),
    ))
service = kubernetes.core.v1.Service("service",
    metadata=kubernetes.meta.v1.ObjectMetaArrgs(
        namespace=nginx_demo.metadata.name,
        labels={
            "app.kubernetes.io/name": "nginx-demo",
        },
    ),
    spec=kubernetes.core.v1.ServiceSpecArrgs(
        type="ClusterIP",
        ports=[kubernetes.core.v1.ServicePortArrgs(
            port=80,
            target_port=80,
            protocol="TCP",
        )],
        selector={
            "app.kubernetes.io/name": "nginx-demo",
        },
    ))
ingress = kubernetes.networking.v1.Ingress("ingress",
    metadata=kubernetes.meta.v1.ObjectMetaArrgs(
        namespace=nginx_demo.metadata.name,
    ),
    spec=kubernetes.networking.v1.IngressSpecArrgs(
        rules=[kubernetes.networking.v1.IngressRuleArrgs(
            host=hostname,
            http=kubernetes.networking.v1.HTTPIngressRuleValueArrgs(
                paths=[kubernetes.networking.v1.HTTPIngressPathArrgs(
                    path="/",
                    path_type="Prefix",
                    backend=kubernetes.networking.v1.IngressBackendArrgs(
                        service=kubernetes.networking.v1.IngressServiceBackendArrgs(
                            name=service.metadata.name,
                            port=kubernetes.networking.v1.ServiceBackendPortArrgs(
                                number=80,
                            ),
                        ),
                    ),
                )],
            ),
        )],
    ))
