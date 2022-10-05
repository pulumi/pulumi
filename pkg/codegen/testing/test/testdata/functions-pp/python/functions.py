import pulumi
import base64
import pulumi_aws as aws

encoded = base64.b64encode("haha business".encode()).decode()
decoded = base64.b64decode(encoded.encode()).decode()
joined = "-".join([
    encoded,
    decoded,
    "2",
])
bucket = aws.s3.Bucket("bucket")
encoded2 = bucket.id.apply(lambda id: base64.b64encode(id.encode()).decode())
decoded2 = bucket.id.apply(lambda id: base64.b64decode(id.encode()).decode())
