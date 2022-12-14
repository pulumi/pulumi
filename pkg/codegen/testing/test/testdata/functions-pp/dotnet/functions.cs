using System;
using System.Collections.Generic;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(() => 
{
    var encoded = Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes("haha business"));

    var decoded = System.Text.Encoding.UTF8.GetString(Convert.FromBase64String(encoded));

    var joined = string.Join("-", new[]
    {
        encoded,
        decoded,
        "2",
    });

    var bucket = new Aws.S3.Bucket("bucket");

    var encoded2 = bucket.Id.Apply(id => Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(id)));

    var decoded2 = bucket.Id.Apply(id => System.Text.Encoding.UTF8.GetString(Convert.FromBase64String(id)));

});

