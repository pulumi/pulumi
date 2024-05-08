import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const argocd_serverDeployment = new kubernetes.apps.v1.Deployment("argocd_serverDeployment", {
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {
        name: "argocd-server",
    },
    spec: {
        selector: {
            matchLabels: {
                app: "server",
            },
        },
        replicas: 1,
        template: {
            metadata: {
                labels: {
                    app: "server",
                },
            },
            spec: {
                containers: [{
                    name: "nginx",
                    image: "nginx",
                    readinessProbe: {
                        httpGet: {
                            port: 8080,
                        },
                    },
                }],
            },
        },
    },
});
