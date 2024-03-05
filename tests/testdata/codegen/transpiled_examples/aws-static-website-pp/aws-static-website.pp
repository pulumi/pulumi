resource siteBucket "aws-native:s3:Bucket" {
	__logicalName = "site-bucket"
	websiteConfiguration = {
		indexDocument = "index.html"
	}
}

resource indexHtml "aws:s3/bucketObject:BucketObject" {
	__logicalName = "index.html"
	bucket = siteBucket
	source = fileAsset("./www/index.html")
	acl = "public-read"
	contentType = "text/html"
}

resource faviconPng "aws:s3/bucketObject:BucketObject" {
	__logicalName = "favicon.png"
	bucket = siteBucket
	source = fileAsset("./www/favicon.png")
	acl = "public-read"
	contentType = "image/png"
}

resource bucketPolicy "aws:s3/bucketPolicy:BucketPolicy" {
	__logicalName = "bucketPolicy"
	bucket = siteBucket.id
	policy = "{\n  \"Version\": \"2012-10-17\",\n  \"Statement\": [\n    {\n      \"Effect\": \"Allow\",\n      \"Principal\": \"*\",\n      \"Action\": [\"s3:GetObject\"],\n      \"Resource\": [\"${siteBucket.arn}/*\"]\n    }\n  ]\n}\n"
}

output bucketName {
	__logicalName = "bucketName"
	value = siteBucket.bucketName
}

output websiteUrl {
	__logicalName = "websiteUrl"
	value = siteBucket.websiteURL
}
