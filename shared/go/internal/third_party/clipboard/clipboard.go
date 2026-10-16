// Copyright 2013 @atotto. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package clipboard read/write on clipboard
//
// Vendored unmodified from github.com/tgummerer/clipboard@d02d263e614e
// (BSD-3-Clause, see LICENSE in this directory), a fork of
// github.com/atotto/clipboard@v0.1.4 whose only change is loading user32.dll
// lazily on Windows instead of in package init. atotto/clipboard's init-time
// load crashes the whole process when user32.dll cannot be loaded (e.g. under
// desktop heap exhaustion), and since it runs at init there is no error path.
// The fork cannot be required directly because its go.mod still declares the
// atotto module path, and a replace directive would not propagate to
// downstream modules that build against this SDK. See
// https://github.com/pulumi/pulumi/issues/19342.
package clipboard

// ReadAll read string from clipboard
func ReadAll() (string, error) {
	return readAll()
}

// WriteAll write string to clipboard
func WriteAll(text string) error {
	return writeAll(text)
}

// Unsupported might be set true during clipboard init, to help callers decide
// whether or not to offer clipboard options.
var Unsupported bool
