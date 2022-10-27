resource bucket "aws:s3:Bucket" {
	options {
	    range = 10
	}
	website = {
		indexDocument = "index-${range.value}.html"
	}
}