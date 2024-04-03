import pulumi
import base64
import hashlib
import mimetypes
import os
import pulumi_aws as aws

def computeFilebase64sha256(path):
	fileData = open(path).read().encode()
	hashedData = hashlib.sha256(fileData.encode()).digest()
	return base64.b64encode(hashedData).decode()

encoded = base64.b64encode("haha business".encode()).decode()
decoded = base64.b64decode(encoded.encode()).decode()
joined = "-".join([
    encoded,
    decoded,
    "2",
])
# tests that we initialize "var, err" with ":=" first, then "=" subsequently (Go specific)
zone = aws.get_availability_zones()
zone2 = aws.get_availability_zones()
bucket = aws.s3.Bucket("bucket")
encoded2 = bucket.id.apply(lambda id: base64.b64encode(id.encode()).decode())
decoded2 = bucket.id.apply(lambda id: base64.b64decode(id.encode()).decode())
secret_value = pulumi.Output.secret("hello")
plain_value = pulumi.Output.unsecret(secret_value)
current_stack = pulumi.get_stack()
current_project = pulumi.get_project()
working_directory = os.getcwd()
file_mime_type = mimetypes.guess_type("./base64.txt")[0]
# using the filebase64 function
first = aws.s3.BucketObject("first",
    bucket=bucket.id,
    source=pulumi.StringAsset((lambda path: base64.b64encode(open(path).read().encode()).decode())("./base64.txt")),
    content_type=file_mime_type,
    tags={
        "stack": current_stack,
        "project": current_project,
        "cwd": working_directory,
    })
# using the filebase64sha256 function
second = aws.s3.BucketObject("second",
    bucket=bucket.id,
    source=pulumi.StringAsset(computeFilebase64sha256("./base64.txt")))
# using the sha1 function
third = aws.s3.BucketObject("third",
    bucket=bucket.id,
    source=pulumi.StringAsset(hashlib.sha1("content".encode()).hexdigest()))
