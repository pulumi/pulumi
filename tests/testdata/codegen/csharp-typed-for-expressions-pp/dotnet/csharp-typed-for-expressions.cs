using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var egress = config.RequireObject<Egress[]>("egress");
    var tags = config.GetObject<Dictionary<string, string>>("tags") ?? null;
    var defaultVpc = Aws.Ec2.GetVpc.Invoke(new()
    {
        Default = true,
    });

    // Create a security group that permits HTTP ingress and unrestricted egress.
    var webSecurityGroup = new Aws.Ec2.SecurityGroup("webSecurityGroup", new()
    {
        VpcId = defaultVpc.Apply(getVpcResult => getVpcResult.Id),
        Egress = egress.Select((v, k) => new { Key = k, Value = v }).Select(entry => 
        {
            return new Aws.Ec2.Inputs.SecurityGroupEgressArgs
            {
                Protocol = "-1",
                FromPort = entry.Value.FromPort,
                ToPort = entry.Value.ToPort,
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
            };
        }).ToList(),
        Tags = tags.Select(pair => new { pair.Key, pair.Value }).ToDictionary(item => {
            var k = item.Key;
            return k;
        }, item => {
            var v = item.Value;
            return v;
        }),
    });

});

public class Egress
{
    public int FromPort { get; set; }
    public int ToPort { get; set; }
}

