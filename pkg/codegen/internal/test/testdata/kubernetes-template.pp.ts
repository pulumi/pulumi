import * as pulumi from "@pulumi/pulumi";
import * as kubernetes from "@pulumi/kubernetes";

const argocd_serverDeployment = new kubernetes.apps.v1.Deployment("argocd_serverDeployment", {
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {
        name: "argocd-server",
    },
    spec: {
        template: {
            spec: {
                containers: [{
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
