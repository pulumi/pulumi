using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var dbCluster = new Aws.Rds.Cluster("dbCluster", new()
    {
        MasterPassword = Output.CreateSecret("foobar"),
    });

});

