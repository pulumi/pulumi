import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const config = new pulumi.Config();
const hostname = config.get("hostname") || "example.com";
const nginxDemo = new kubernetes.core.v1.Namespace("nginx-demo", {});
const app = new kubernetes.apps.v1.Deployment("app", {
    metadata: {
        namespace: nginxDemo.metadata.apply(metadata => metadata?.name),
    },
    spec: {
        selector: {
            matchLabels: {
                "app.kubernetes.io/name": "nginx-demo",
            },
        },
        replicas: 1,
        template: {
            metadata: {
                labels: {
                    "app.kubernetes.io/name": "nginx-demo",
                },
            },
            spec: {
                containers: [{
                    name: "app",
                    image: "nginx:1.15-alpine",
                }],
            },
        },
    },
});
const service = new kubernetes.core.v1.Service("service", {
    metadata: {
        namespace: nginxDemo.metadata.apply(metadata => metadata?.name),
        labels: {
            "app.kubernetes.io/name": "nginx-demo",
        },
    },
    spec: {
        type: kubernetes.core.v1.ServiceSpecType.ClusterIP,
        ports: [{
            port: 80,
            targetPort: 80,
            protocol: "TCP",
        }],
        selector: {
            "app.kubernetes.io/name": "nginx-demo",
        },
    },
});
const ingress = new kubernetes.networking.v1.Ingress("ingress", {
    metadata: {
        namespace: nginxDemo.metadata.apply(metadata => metadata?.name),
    },
    spec: {
        rules: [{
            host: hostname,
            http: {
                paths: [{
                    path: "/",
                    pathType: "Prefix",
                    backend: {
                        service: {
                            name: service.metadata.apply(metadata => metadata?.name),
                            port: {
                                number: 80,
                            },
                        },
                    },
                }],
            },
        }],
    },
});
