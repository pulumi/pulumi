import pulumi
import pulumi_aws as aws
import pulumi_aws_native as aws_native

site_bucket = aws_native.s3.Bucket("site-bucket", website_configuration={
    "index_document": "index.html",
})
index_html = aws.s3.BucketObject("index.html",
    bucket=site_bucket,
    source=pulumi.FileAsset("./www/index.html"),
    acl="public-read",
    content_type="text/html")
favicon_png = aws.s3.BucketObject("favicon.png",
    bucket=site_bucket,
    source=pulumi.FileAsset("./www/favicon.png"),
    acl="public-read",
    content_type="image/png")
bucket_policy = aws.s3.BucketPolicy("bucketPolicy",
    bucket=site_bucket.id,
    policy=site_bucket.arn.apply(lambda arn: f"""{{
  "Version": "2012-10-17",
  "Statement": [
    {{
      "Effect": "Allow",
      "Principal": "*",
      "Action": ["s3:GetObject"],
      "Resource": ["{arn}/*"]
    }}
  ]
}}
"""))
pulumi.export("bucketName", site_bucket.bucket_name)
pulumi.export("websiteUrl", site_bucket.website_url)
