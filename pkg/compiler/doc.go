// Copyright 2016 Marapongo, Inc. All rights reserved.

package compiler

import (
	"github.com/marapongo/mu/pkg/schema"
)

type Document struct {
	File  string        // the file that this document refers to.
	Stack *schema.Stack // the root stack element inside of this document.
}
