import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";
import * as awsx from "@pulumi/awsx";

const cluster = new aws.ecs.Cluster("cluster", {});
const lb = new awsx.lb.ApplicationLoadBalancer("lb", {});
const nginx = new awsx.ecs.FargateService("nginx", {
    cluster: cluster.arn,
    taskDefinitionArgs: {
        container: {
            image: "nginx:latest",
            cpu: 512,
            memory: 128,
            portMappings: [{
                containerPort: 80,
                targetGroup: lb.defaultTargetGroup,
            }],
        },
    },
});
export const url = lb.loadBalancer.dnsName;
