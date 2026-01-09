import pulumi
import pulumi_aws as aws

site_bucket = aws.s3.Bucket("siteBucket")
test_file_asset = aws.s3.BucketObject("testFileAsset",
    bucket=site_bucket.id,
    source=pulumi.FileAsset("file.txt"))
test_string_asset = aws.s3.BucketObject("testStringAsset",
    bucket=site_bucket.id,
    source=pulumi.StringAsset("<h1>File contents</h1>"))
test_remote_asset = aws.s3.BucketObject("testRemoteAsset",
    bucket=site_bucket.id,
    source=pulumi.RemoteAsset("https://pulumi.test"))
test_file_archive = aws.lambda_.Function("testFileArchive",
    role=site_bucket.arn,
    code=pulumi.FileArchive("file.tar.gz"))
test_remote_archive = aws.lambda_.Function("testRemoteArchive",
    role=site_bucket.arn,
    code=pulumi.RemoteArchive("https://pulumi.test/foo.tar.gz"))
test_asset_archive = aws.lambda_.Function("testAssetArchive",
    role=site_bucket.arn,
    code=pulumi.AssetArchive({
        "file.txt": pulumi.FileAsset("file.txt"),
        "string.txt": pulumi.StringAsset("<h1>File contents</h1>"),
        "remote.txt": pulumi.RemoteAsset("https://pulumi.test"),
        "file.tar": pulumi.FileArchive("file.tar.gz"),
        "remote.tar": pulumi.RemoteArchive("https://pulumi.test/foo.tar.gz"),
        ".nestedDir": pulumi.AssetArchive({
            "file.txt": pulumi.FileAsset("file.txt"),
            "string.txt": pulumi.StringAsset("<h1>File contents</h1>"),
            "remote.txt": pulumi.RemoteAsset("https://pulumi.test"),
            "file.tar": pulumi.FileArchive("file.tar.gz"),
            "remote.tar": pulumi.RemoteArchive("https://pulumi.test/foo.tar.gz"),
        }),
    }))
