using System.Collections.Generic;
using Pulumi;
using Docker = Pulumi.Docker;

return await Deployment.RunAsync(() => 
{
    var latest = Docker.GetRemoteImage.Invoke(new()
    {
        Name = "nginx",
    });

    var ubuntu = new Docker.RemoteImage("ubuntu", new()
    {
        Name = "ubuntu:precise",
    });

    return new Dictionary<string, object?>
    {
        ["remoteImageId"] = latest.Apply(getRemoteImageResult => getRemoteImageResult.Id),
        ["ubuntuImage"] = ubuntu.Name,
    };
});

