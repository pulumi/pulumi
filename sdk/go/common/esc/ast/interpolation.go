// Copyright 2023, Pulumi Corporation.  All rights reserved.

package ast

import (
	"strings"

	"github.com/pulumi/environments/syntax"
)

type Interpolation struct {
	Text  string
	Value *PropertyAccess
}

func parseInterpolate(node syntax.Node, value string) ([]Interpolation, syntax.Diagnostics) {
	var parts []Interpolation
	var str strings.Builder
	for len(value) > 0 {
		switch {
		case strings.HasPrefix(value, "$$"):
			str.WriteByte('$')
			value = value[2:]
		case strings.HasPrefix(value, "${"):
			rest, access, diags := parsePropertyAccess(node, value[2:])
			if len(diags) != 0 {
				return nil, diags
			}
			parts = append(parts, Interpolation{
				Text:  str.String(),
				Value: access,
			})
			str.Reset()

			value = rest
		default:
			str.WriteByte(value[0])
			value = value[1:]
		}
	}
	if str.Len() != 0 {
		parts = append(parts, Interpolation{Text: str.String()})
	}
	return parts, nil
}
