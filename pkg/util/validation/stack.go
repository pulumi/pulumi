package validation

import validation "github.com/pulumi/pulumi/sdk/v3/pkg/util/validation"

// ValidateStackTags validates the tag names and values.
func ValidateStackTags(tags map[apitype.StackTagName]string) error {
	return validation.ValidateStackTags(tags)
}

