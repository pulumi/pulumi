using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using Pulumi;
using Aws = Pulumi.Aws;

	
string ComputeFileBase64Sha256(string path) 
{
    var fileData = Encoding.UTF8.GetBytes(File.ReadAllText(path));
    var hashData = SHA256.Create().ComputeHash(fileData);
    return Convert.ToBase64String(hashData);
}

	
string ComputeSHA1(string input) 
{
    var hash = SHA1.Create().ComputeHash(Encoding.UTF8.GetBytes(input));
    return BitConverter.ToString(hash).Replace("-","").ToLowerInvariant();
}

	
string ReadFileBase64(string path) 
{
    return Convert.ToBase64String(Encoding.UTF8.GetBytes(File.ReadAllText(path)));
}

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

    // tests that we initialize "var, err" with ":=" first, then "=" subsequently (Go specific)
    var zone = Aws.GetAvailabilityZones.Invoke();

    var zone2 = Aws.GetAvailabilityZones.Invoke();

    var bucket = new Aws.S3.Bucket("bucket");

    var encoded2 = bucket.Id.Apply(id => Convert.ToBase64String(System.Text.Encoding.UTF8.GetBytes(id)));

    var decoded2 = bucket.Id.Apply(id => System.Text.Encoding.UTF8.GetString(Convert.FromBase64String(id)));

    var secretValue = Output.CreateSecret("hello");

    var plainValue = Output.Unsecret(secretValue);

    var currentStack = Deployment.Instance.StackName;

    var currentProject = Deployment.Instance.ProjectName;

    var workingDirectory = Directory.GetCurrentDirectory();

    var fileMimeType = "TODO: call mimeType";

    // using the filebase64 function
    var first = new Aws.S3.BucketObject("first", new()
    {
        Bucket = bucket.Id,
        Source = new StringAsset(ReadFileBase64("./base64.txt")),
        ContentType = fileMimeType,
        Tags = 
        {
            { "stack", currentStack },
            { "project", currentProject },
            { "cwd", workingDirectory },
        },
    });

    // using the filebase64sha256 function
    var second = new Aws.S3.BucketObject("second", new()
    {
        Bucket = bucket.Id,
        Source = new StringAsset(ComputeFileBase64Sha256("./base64.txt")),
    });

    // using the sha1 function
    var third = new Aws.S3.BucketObject("third", new()
    {
        Bucket = bucket.Id,
        Source = new StringAsset(ComputeSHA1("content")),
    });

});

