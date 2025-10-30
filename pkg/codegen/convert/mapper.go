package convert

import convert "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/convert"

// Mapper provides methods for retrieving mappings that describe how to map names in some source "provider" (e.g. a
// Terraform provider, if we are converting from Terraform) to names in appropriate Pulumi packages. So when converting
// a Terraform program containing code like `resource "aws_s3_bucket" "b" {}`, for instance, we need to know (among
// other things) that the `aws_s3_bucket` Terraform resource type corresponds to the Pulumi type `aws:s3/bucket:Bucket`,
// and thus lives in the `aws` package. This is the kind of information that a Mapper provides.
type Mapper = convert.Mapper

// MapperPackageHint is the type of hints that may be passed to GetMapping to help guide implementations to picking
// appropriate Pulumi packages to satisfy mapping requests.
type MapperPackageHint = convert.MapperPackageHint

