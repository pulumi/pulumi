// Copyright 2016 Pulumi, Inc. All rights reserved.

package awsctx

import (
	"errors"
	"fmt"

	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
)

// MaybeCheckError produces a non-nil error if the given array contains 1 or more failures; nil otherwise.
func MaybeCheckError(failures []*cocorpc.CheckFailure) error {
	if len(failures) == 0 {
		return nil
	}

	str := fmt.Sprintf("%d of this security group rule's properties failed validation:", len(failures))
	for _, failure := range failures {
		str += fmt.Sprintf("\n\t%v: %v", failure.Property, failure.Reason)
	}
	return errors.New(str)
}
