using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    // Create a new security group for port 80.
    var securityGroup = new Aws.Ec2.SecurityGroup("securityGroup", new()
    {
        Ingress = new[]
        {
            new Aws.Ec2.Inputs.SecurityGroupIngressArgs
            {
                Protocol = "tcp",
                FromPort = 0,
                ToPort = 0,
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
            },
        },
    });

    // Get the ID for the latest Amazon Linux AMI.
    var ami = Aws.GetAmi.Invoke(new()
    {
        Filters = new[]
        {
            new Aws.Inputs.GetAmiFilterInputArgs
            {
                Name = "name",
                Values = new[]
                {
                    "amzn-ami-hvm-*-x86_64-ebs",
                },
            },
        },
        Owners = new[]
        {
            "137112412989",
        },
        MostRecent = true,
    });

    // Create a simple web server using the startup script for the instance.
    var server = new Aws.Ec2.Instance("server", new()
    {
        Tags = 
        {
            { "Name", "web-server-www" },
        },
        InstanceType = Aws.Ec2.InstanceType.T2_Micro,
        SecurityGroups = new[]
        {
            securityGroup.Name,
        },
        Ami = ami.Apply(getAmiResult => getAmiResult.Id),
        UserData = @"#!/bin/bash
echo ""Hello, World!"" > index.html
nohup python -m SimpleHTTPServer 80 &
",
    });

    return new Dictionary<string, object?>
    {
        ["publicIp"] = server.PublicIp,
        ["publicHostName"] = server.PublicDns,
    };
});

