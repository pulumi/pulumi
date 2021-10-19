using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var aws_vpc = new Aws.Ec2.Vpc("aws_vpc", new Aws.Ec2.VpcArgs
        {
            CidrBlock = "10.0.0.0/16",
            InstanceTenancy = "default",
        });
        var privateS3VpcEndpoint = new Aws.Ec2.VpcEndpoint("privateS3VpcEndpoint", new Aws.Ec2.VpcEndpointArgs
        {
            VpcId = aws_vpc.Id,
            ServiceName = "com.amazonaws.us-west-2.s3",
        });
        var privateS3PrefixList = Aws.Ec2.GetPrefixList.Invoke(new Aws.Ec2.GetPrefixListInvokeArgs
        {
            PrefixListId = privateS3VpcEndpoint.PrefixListId,
        });
        var bar = new Aws.Ec2.NetworkAcl("bar", new Aws.Ec2.NetworkAclArgs
        {
            VpcId = aws_vpc.Id,
        });
        var privateS3NetworkAclRule = new Aws.Ec2.NetworkAclRule("privateS3NetworkAclRule", new Aws.Ec2.NetworkAclRuleArgs
        {
            NetworkAclId = bar.Id,
            RuleNumber = 200,
            Egress = false,
            Protocol = "tcp",
            RuleAction = "allow",
            CidrBlock = privateS3PrefixList.Apply(privateS3PrefixList => privateS3PrefixList.CidrBlocks[0]),
            FromPort = 443,
            ToPort = 443,
        });
        var amis = Aws.Ec2.GetAmiIds.Invoke(new Aws.Ec2.GetAmiIdsInvokeArgs
        {
            Owners = 
            {
                bar.Id,
            },
            Filters = 
            {
                new Aws.Ec2.Inputs.GetAmiIdsFilterInputArgs
                {
                    Name = bar.Id,
                    Values = 
                    {
                        "pulumi*",
                    },
                },
            },
        });
    }

}
