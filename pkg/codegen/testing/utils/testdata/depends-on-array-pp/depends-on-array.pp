resource myBucket "aws:s3/bucket:Bucket" {
	website = {
		indexDocument = "index.html"
	}
}

resource ownershipControls "aws:s3/bucketOwnershipControls:BucketOwnershipControls" {
	bucket = myBucket.id
	rule = {
		objectOwnership = "ObjectWriter"
	}
}

resource publicAccessBlock "aws:s3/bucketPublicAccessBlock:BucketPublicAccessBlock" {
	bucket = myBucket.id
	blockPublicAcls = false
}

resource indexHtml "aws:s3/bucketObject:BucketObject" {
	__logicalName = "index.html"
	bucket = myBucket.id
	source = fileAsset("./index.html")
	contentType = "text/html"
	acl = "public-read"

	options {
		dependsOn = [
			publicAccessBlock,
			ownershipControls
		]
	}
}

output bucketName {
	value = myBucket.id
}

output bucketEndpoint {
	value = "http://${myBucket.websiteEndpoint}"
}
