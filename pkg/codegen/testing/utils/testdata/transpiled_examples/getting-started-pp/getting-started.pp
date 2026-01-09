resource mybucket "aws:s3/bucket:Bucket" {
	website = {
		indexDocument = "index.html"
	}
}

resource indexhtml "aws:s3/bucketObject:BucketObject" {
	bucket = mybucket.id
	source = stringAsset("<h1>Hello, world!</h1>")
	acl = "public-read"
	contentType = "text/html"
}

output bucketEndpoint {
	value = "http://${mybucket.websiteEndpoint}"
}
