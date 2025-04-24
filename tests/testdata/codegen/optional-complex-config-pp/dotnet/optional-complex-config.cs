using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    // The tag of the VPC
    var vpcTag = config.Get("vpcTag");
    // The id of a VPC to use instead of creating a new one
    var vpcId = config.Get("vpcId");
    // The list of subnets to use
    var subnets = config.GetObject<string[]>("subnets");
    // Additional tags to add to the VPC
    var moreTags = config.GetObject<Dictionary<string, string>>("moreTags");
    // The userdata to use for the instances
    var userdata = config.GetObject<Userdata>("userdata");
    // A complex object
    var complexUserdata = config.GetObject<ComplexUserdata[]>("complexUserdata");
    var main = new Aws.Ec2.Vpc("main", new()
    {
        CidrBlock = "10.100.0.0/16",
        Tags = 
        {
            { "Name", vpcTag },
        },
    });

});

public class ComplexUserdata
{
    public string content { get; set; }
    public string path { get; set; }
}

public class Userdata
{
    public string content { get; set; }
    public string path { get; set; }
}

