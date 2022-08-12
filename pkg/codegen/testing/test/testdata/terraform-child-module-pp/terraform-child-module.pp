# Local Terraform Child Module

resource s3Bucket "aws:s3:Bucket" {
  bucket = "s3BucketId"
}

module childModule {
  customModuleParameter = s3Bucket.id
  source = "./modules/aws-s3-static-website-bucket"
}
