using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var test = new Aws.Fsx.OpenZfsFileSystem("test", new()
    {
        StorageCapacity = 64,
        SubnetIds = new[]
        {
            aws_subnet.Test1.Id,
        },
        DeploymentType = "SINGLE_AZ_1",
        ThroughputCapacity = 64,
    });

});

