import pulumi
import pulumi_aws as aws

aws_vpc = aws.ec2.Vpc("aws_vpc",
    cidr_block="10.0.0.0/16",
    instance_tenancy="default")
private_s3_vpc_endpoint = aws.ec2.VpcEndpoint("privateS3VpcEndpoint",
    vpc_id=aws_vpc.id,
    service_name="com.amazonaws.us-west-2.s3")
private_s3_prefix_list = aws.ec2.get_prefix_list_output(prefix_list_id=private_s3_vpc_endpoint.prefix_list_id)
bar = aws.ec2.NetworkAcl("bar", vpc_id=aws_vpc.id)
private_s3_network_acl_rule = aws.ec2.NetworkAclRule("privateS3NetworkAclRule",
    network_acl_id=bar.id,
    rule_number=200,
    egress=False,
    protocol="tcp",
    rule_action="allow",
    cidr_block=private_s3_prefix_list.cidr_blocks[0],
    from_port=443,
    to_port=443)
# A contrived example to test that helper nested records ( `filters`
# below) generate correctly when using output-versioned function
# invoke forms.
amis = aws.ec2.get_ami_ids_output(owners=[bar.id],
    filters=[{
        "name": bar.id,
        "values": ["pulumi*"],
    }])
