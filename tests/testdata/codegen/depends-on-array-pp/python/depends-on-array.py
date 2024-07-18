import pulumi
import pulumi_aws as aws

my_bucket = aws.s3.Bucket("myBucket", website={
    "index_document": "index.html",
})
ownership_controls = aws.s3.BucketOwnershipControls("ownershipControls",
    bucket=my_bucket.id,
    rule={
        "object_ownership": "ObjectWriter",
    })
public_access_block = aws.s3.BucketPublicAccessBlock("publicAccessBlock",
    bucket=my_bucket.id,
    block_public_acls=False)
index_html = aws.s3.BucketObject("index.html",
    bucket=my_bucket.id,
    source=pulumi.FileAsset("./index.html"),
    content_type="text/html",
    acl="public-read",
    opts = pulumi.ResourceOptions(depends_on=[
            public_access_block,
            ownership_controls,
        ]))
pulumi.export("bucketName", my_bucket.id)
pulumi.export("bucketEndpoint", my_bucket.website_endpoint.apply(lambda website_endpoint: f"http://{website_endpoint}"))
