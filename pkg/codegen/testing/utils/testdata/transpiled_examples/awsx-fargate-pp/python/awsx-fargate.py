import pulumi
import pulumi_aws as aws
import pulumi_awsx as awsx

cluster = aws.ecs.Cluster("cluster")
lb = awsx.lb.ApplicationLoadBalancer("lb")
nginx = awsx.ecs.FargateService("nginx",
    cluster=cluster.arn,
    task_definition_args={
        "container": {
            "image": "nginx:latest",
            "cpu": 512,
            "memory": 128,
            "port_mappings": [{
                "container_port": 80,
                "target_group": lb.default_target_group,
            }],
        },
    })
pulumi.export("url", lb.load_balancer.dns_name)
