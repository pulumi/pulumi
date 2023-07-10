config "azCount" "string" {
  default     = "10"
  description = "Number of AZs to cover in a given region"
}

resource bucketsPerAvailabilityZone "aws:s3:Bucket" {
    options {
        // using the azCount config variable which is a string
        range = azCount
    }

    website = {
    	indexDocument = "index.html"
    }
}