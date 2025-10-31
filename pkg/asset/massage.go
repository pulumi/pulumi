package asset

import asset "github.com/pulumi/pulumi/sdk/v3/pkg/asset"

// IsUserProgramCode checks to see if this is the special asset containing the users's code
func IsUserProgramCode(a *resource.Asset) bool {
	return asset.IsUserProgramCode(a)
}

// MassageIfUserProgramCodeAsset takes the text for a function and cleans it up a bit to make the
// user visible diffs less noisy.  Specifically:
//  1. it tries to condense things by changling multiple blank lines into a single blank line.
//  2. it normalizs the sha hashes we emit so that changes to them don't appear in the diff.
//  3. it elides the with-capture headers, as changes there are not generally meaningful.
// 
// TODO(https://github.com/pulumi/pulumi/issues/592) this is baking in a lot of knowledge about
// pulumi serialized functions.  We should try to move to an alternative mode that isn't so brittle.
// Options include:
//  1. Have a documented delimeter format that plan.go will look for.  Have the function serializer
//     emit those delimeters around code that should be ignored.
//  2. Have our resource generation code supply not just the resource, but the "user presentable"
//     resource that cuts out a lot of cruft.  We could then just diff that content here.
func MassageIfUserProgramCodeAsset(asset_ *resource.Asset, debug bool) *resource.Asset {
	return asset.MassageIfUserProgramCodeAsset(asset_, debug)
}

