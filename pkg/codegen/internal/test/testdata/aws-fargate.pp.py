import pulumi
import json
import pulumi_aws as aws

vpc = aws.ec2.get_vpc(default=True)
subnets = aws.ec2.get_subnet_ids(vpc_id=vpc.id)
# Create a security group that permits HTTP ingress and unrestricted egress.
web_security_group = aws.ec2.SecurityGroup("webSecurityGroup",
    vpc_id=vpc.id,
    egress=[aws.ec2.SecurityGroupEgressArgs(
        protocol="-1",
        from_port=0,
        to_port=0,
        cidr_blocks=["0.0.0.0/0"],
    )],
    ingress=[aws.ec2.SecurityGroupIngressArgs(
        protocol="tcp",
        from_port=80,
        to_port=80,
        cidr_blocks=["0.0.0.0/0"],
    )])
# Create an ECS cluster to run a container-based service.
cluster = aws.ecs.Cluster("cluster")
# Create an IAM role that can be used by our service's task.
task_exec_role = aws.iam.Role("taskExecRole", assume_role_policy=json.dumps({
    "Version": "2008-10-17",
    "Statement": [{
        "Sid": "",
        "Effect": "Allow",
        "Principal": {
            "Service": "ecs-tasks.amazonaws.com",
        },
        "Action": "sts:AssumeRole",
    }],
}))
task_exec_role_policy_attachment = aws.iam.RolePolicyAttachment("taskExecRolePolicyAttachment",
    role=task_exec_role.name,
    policy_arn="arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy")
# Create a load balancer to listen for HTTP traffic on port 80.
web_load_balancer = aws.elasticloadbalancingv2.LoadBalancer("webLoadBalancer",
    subnets=subnets.ids,
    security_groups=[web_security_group.id])
web_target_group = aws.elasticloadbalancingv2.TargetGroup("webTargetGroup",
    port=80,
    protocol="HTTP",
    target_type="ip",
    vpc_id=vpc.id)
web_listener = aws.elasticloadbalancingv2.Listener("webListener",
    load_balancer_arn=web_load_balancer.arn,
    port=80,
    default_actions=[aws.elasticloadbalancingv2.ListenerDefaultActionArgs(
        type="forward",
        target_group_arn=web_target_group.arn,
    )])
# Spin up a load balanced service running NGINX
app_task = aws.ecs.TaskDefinition("appTask",
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
app_service = aws.ecs.Service("appService",
    cluster=cluster.arn,
    desired_count=5,
    launch_type="FARGATE",
    task_definition=app_task.arn,
    network_configuration=aws.ecs.ServiceNetworkConfigurationArgs(
        assign_public_ip=True,
        subnets=subnets.ids,
        security_groups=[web_security_group.id],
    ),
    load_balancers=[aws.ecs.ServiceLoadBalancerArgs(
        target_group_arn=web_target_group.arn,
        container_name="my-app",
        container_port=80,
    )],
    opts=pulumi.ResourceOptions(depends_on=[web_listener]))
pulumi.export("url", web_load_balancer.dns_name)
