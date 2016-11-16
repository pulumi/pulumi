// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/schema"
)

// Context holds all state available to any templates or code evaluated at compile-time.
type Context struct {
	Args   map[string]interface{}
	Target *schema.Target
}
