// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package ioutil

import (
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

// MustFprintf works like fmt.Fprintf, but asserts that the returned error is nil.
func MustFprintf(w io.Writer, format string, a ...interface{}) int {
	n, err := fmt.Fprintf(w, format, a...)
	contract.Assertf(err == nil, "fmt.Fprintf returned non nil error %v", err)
	return n
}
