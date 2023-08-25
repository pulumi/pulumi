// Copyright 2022, Pulumi Corporation.  All rights reserved.

package syntax

import "github.com/hashicorp/hcl/v2"

type Syntax interface {
	Range() *hcl.Range
}

var NoSyntax = noSyntax(0)

type noSyntax int

func (noSyntax) Range() *hcl.Range {
	return nil
}

type Trivia interface {
	Syntax

	HeadComment() string
	LineComment() string
	FootComment() string
}
