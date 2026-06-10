import * as pulumi from "@pulumi/pulumi";
import * as manifest from "@pulumi/manifest";

const first = new manifest.Resource("first", {
    kind: "Manifest",
    metadata: {
        name: "first",
        labels: {
            app: "first",
        },
    },
    spec: {
        replicas: 1,
        template: {
            metadata: {
                name: "inner",
            },
            containers: [{
                name: "app",
                image: "nginx",
                ports: [80],
            }],
        },
    },
});
export const kind = first.kind;
