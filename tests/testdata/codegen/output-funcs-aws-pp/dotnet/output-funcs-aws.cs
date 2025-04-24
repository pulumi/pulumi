using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var aws_vpc = new Aws.Ec2.Vpc("aws_vpc", new()
    {
        CidrBlock = "10.0.0.0/16",
        InstanceTenancy = "default",
    });

    var privateS3VpcEndpoint = new Aws.Ec2.VpcEndpoint("privateS3VpcEndpoint", new()
    {
        VpcId = aws_vpc.Id,
        ServiceName = "com.amazonaws.us-west-2.s3",
    });

    var privateS3PrefixList = Aws.Ec2.GetPrefixList.Invoke(new()
    {
        PrefixListId = privateS3VpcEndpoint.PrefixListId,
    });

    var bar = new Aws.Ec2.NetworkAcl("bar", new()
    {
        VpcId = aws_vpc.Id,
    });

    var privateS3NetworkAclRule = new Aws.Ec2.NetworkAclRule("privateS3NetworkAclRule", new()
    {
        NetworkAclId = bar.Id,
        RuleNumber = 200,
        Egress = false,
        Protocol = "tcp",
        RuleAction = "allow",
        CidrBlock = privateS3PrefixList.Apply(getPrefixListResult => getPrefixListResult.CidrBlocks[0]),
        FromPort = 443,
        ToPort = 443,
    });

    // A contrived example to test that helper nested records ( `filters`
    // below) generate correctly when using output-versioned function
    // invoke forms.
    var amis = Aws.Ec2.GetAmiIds.Invoke(new()
    {
        Owners = new[]
        {
            bar.Id,
        },
        Filters = new[]
        {
            new Aws.Ec2.Inputs.GetAmiIdsFilterInputArgs
            {
                Name = bar.Id,
                Values = new[]
                {
                    "pulumi*",
                },
            },
        },
    });

});

