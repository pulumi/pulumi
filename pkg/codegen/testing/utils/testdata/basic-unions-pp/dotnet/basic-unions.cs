using System.Collections.Generic;
using System.Linq;
using Pulumi;
using BasicUnions = Pulumi.BasicUnions;

return await Deployment.RunAsync(() => 
{
    // properties field is bound to union case ServerPropertiesForReplica
    var replica = new BasicUnions.ExampleServer("replica", new()
    {
        Properties = new BasicUnions.Inputs.ServerPropertiesForReplicaArgs
        {
            CreateMode = "Replica",
            Version = "0.1.0-dev",
        },
    });

    // properties field is bound to union case ServerPropertiesForRestore
    var restore = new BasicUnions.ExampleServer("restore", new()
    {
        Properties = new BasicUnions.Inputs.ServerPropertiesForRestoreArgs
        {
            CreateMode = "PointInTimeRestore",
            RestorePointInTime = "example",
        },
    });

});

