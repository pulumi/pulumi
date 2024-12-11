using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var explicitProvider = new Aws.Provider("explicitProvider", new()
    {
        Region = "us-west-2",
    });

    var zone = Aws.GetAvailabilityZones.Invoke(new()
    {
        AllAvailabilityZones = true,
    }, new InvokeOutputOptions() {
        Provider = explicitProvider,
        Parent = explicitProvider,
        Version = "1.2.3",
        PluginDownloadURL = "https://example.com",
    });

    var server = new Aws.Ec2.Instance("server", new()
    {
        InstanceType = Aws.Ec2.InstanceType.T2_Micro,
    });

    var zoneWithDepends = Aws.GetAvailabilityZones.Invoke(new()
    {
        AllAvailabilityZones = true,
    }, new InvokeOutputOptions() {
        DependsOn = new[]
        {
            server,
        },
    });

});

