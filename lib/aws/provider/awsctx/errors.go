// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package awsctx

import (
	"github.com/aws/aws-sdk-go/aws/awserr"
)

// IsAWSError returns true if the given error is an AWS error; if codes is non-zero in length, an AWS error with any one
// of these codes will return true; if codes is empty, then any AWS error is accepted.
func IsAWSError(err error, codes ...string) bool {
	if erraws, iserraws := err.(awserr.Error); iserraws {
		if len(codes) == 0 {
			return true
		}
		for _, code := range codes {
			if erraws.Code() == code {
				return true
			}
		}
	}
	return false
}

// IsAWSErrorMessage returns true if the given error is an AWS error with the given code and message.
func IsAWSErrorMessage(err error, code string, message string) bool {
	if erraws, iserraws := err.(awserr.Error); iserraws {
		if erraws.Code() == code && erraws.Message() == message {
			return true
		}
	}
	return false
}
