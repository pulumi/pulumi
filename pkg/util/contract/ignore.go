// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package contract

import (
	"io"

	"github.com/pulumi/pulumi/pkg/util/logging"
)

// Ignore explicitly ignores a value.  This is similar to `_ = x`, but tells linters ignoring is intentional.
func Ignore(v interface{}) {
	// Log something at a VERY verbose level just in case it helps to track down issues (e.g., an error that was
	// ignored that represents something even more egregious than the eventual failure mode).  If this truly matters, it
	// probably implies the ignore was not appropriate, but as a safeguard, logging seems useful.
	logging.V(11).Infof("Explicitly ignoring and discarding result: %v", v)
}

// IgnoreClose closes and ignores the returned error.  This makes defer closes easier.
func IgnoreClose(cr io.Closer) {
	err := cr.Close()
	IgnoreError(err)
}

// IgnoreError explicitly ignores an error.  This is similar to `_ = x`, but tells linters ignoring is intentional.
// This routine is specifically for ignoring errors which is potentially more risky, and so logs at a higher level.
func IgnoreError(err error) {
	// Log something at a verbose level just in case it helps to track down issues (e.g., an error that was
	// ignored that represents something even more egregious than the eventual failure mode).  If this truly matters, it
	// probably implies the ignore was not appropriate, but as a safeguard, logging seems useful.
	if err != nil {
		logging.V(3).Infof("Explicitly ignoring and discarding error: %v", err)
	}
}
