// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apitype

import "fmt"

// ErrorResponse is returned from the API when an actual response body is not appropriate. i.e.
// in all error situations.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the Error interface.
func (err ErrorResponse) Error() string {
	return fmt.Sprintf("[%d] %s", err.Code, err.Message)
}
