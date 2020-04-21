import pulumi
import json
import pulumi_aws as aws

vpc = aws.ec2.get_vpc({
    "default": True,
})
subnets = aws.ec2.get_subnet_ids({
    "vpcId": vpc.id,
})
# Create a security group that permits HTTP ingress and unrestricted egress.
web_security_group = aws.ec2.SecurityGroup("web_security_group",
    vpc_id=vpc.id,
    egress=[{
        "protocol": "-1",
        "fromPort": 0,
        "toPort": 0,
        "cidrBlocks": ["0.0.0.0/0"],
    }],
    ingress=[{
        "protocol": "tcp",
        "fromPort": 80,
        "toPort": 80,
        "cidrBlocks": ["0.0.0.0/0"],
    }])
# Create an ECS cluster to run a container-based service.
cluster = aws.ecs.Cluster("cluster")
# Create an IAM role that can be used by our service's task.
task_exec_role = aws.iam.Role("task_exec_role", assume_role_policy={
    "Version": "2008-10-17",
    "Statement": [{
        "Sid": "",
        "Effect": "Allow",
        "Principal": {
            "Service": "ecs-tasks.amazonaws.com",
        },
        "Action": "sts:AssumeRole",
    }],
})
task_exec_role_policy_attachment = aws.iam.RolePolicyAttachment("task_exec_role_policy_attachment",
    role=task_exec_role.name,
    policy_arn="arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy")
# Create a load balancer to listen for HTTP traffic on port 80.
web_load_balancer = aws.elasticloadbalancingv2.LoadBalancer("web_load_balancer",
    subnets=subnets.ids,
    security_groups=[web_security_group.id])
web_target_group = aws.elasticloadbalancingv2.TargetGroup("web_target_group",
    port=80,
    protocol="HTTP",
    target_type="ip",
    vpc_id=vpc.id)
web_listener = aws.elasticloadbalancingv2.Listener("web_listener",
    load_balancer_arn=web_load_balancer.arn,
    port=80,
    default_actions=[{
        "type": "forward",
        "targetGroupArn": web_target_group.arn,
    }])
# Spin up a load balanced service running NGINX
app_task = aws.ecs.TaskDefinition("app_task",
    family="fargate-task-definition",
    cpu="256",
    memory="512",
    network_mode="awsvpc",
    requires_compatibilities=["FARGATE"],
    execution_role_arn=task_exec_role.arn,
    container_definitions=json.dumps([{
        "name": "my-app",
        "image": "nginx",
        "portMappings": [{
            "containerPort": 80,
            "hostPort": 80,
            "protocol": "tcp",
        }],
    }]))
app_service = aws.ecs.Service("app_service",
    cluster=cluster.arn,
    desired_count=5,
    launch_type="FARGATE",
    task_definition=app_task.arn,
    network_configuraiton={
        "assignPublicIp": True,
        "subnets": subnets.ids,
        "securityGroups": [web_security_group.id],
    },
    load_balancers=[{
        "targetGroupArn": web_target_group.arn,
        "containerName": "my-app",
        "containerPort": 80,
    }])
pulumi.export("url", web_load_balancer.dnsName)
