resource provider "pulumi:providers:aws" {
	region = "us-west-2"

	options {
		urnName = "bucket"
	}
}

resource bucket1 "aws:s3:Bucket" {
	options {
		provider = provider
		dependsOn = [provider]
		protect = true
		ignoreChanges = [bucket, lifecycleRules[0]]
		urnName = "bucket"
	}
}
