// Copyright 2016 Marapongo, Inc. All rights reserved.

package metadata

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/errors"
)

func TestBadPrimitives(t *testing.T) {
	sink := buildNoCodegen("testdata", "bad__primitives")

	badarrs := []string{"[]string", "stri[]ng", "[]string", "[]string[]", "[]"}
	badboths := []string{"map[]int[string]", "map[]int[]"}
	badmaps := []string{"map[string", "map[string]", "map[]", "map[]int"}
	assert.Equal(t, len(badarrs)+len(badboths)+len(badmaps), sink.Errors(),
		"expected an error per badarrN/badbothN/badmapN case")

	msg := 0
	{
		d := errors.ErrorIllegalArrayLikeSyntax
		for _, bad := range badarrs {
			assert.Equal(t,
				fmt.Sprintf("%v: %v%v: %v\n",
					diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, bad)),
				sink.ErrorMsgs()[msg])
			msg++
		}
	}

	{
		d := errors.ErrorIllegalMapLikeSyntax
		for _, bad := range badboths {
			assert.Equal(t,
				fmt.Sprintf("%v: %v%v: %v\n",
					diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, bad)),
				sink.ErrorMsgs()[msg])
			msg++
		}
	}

	{
		d := errors.ErrorIllegalMapLikeSyntax
		for _, bad := range badmaps {
			assert.Equal(t,
				fmt.Sprintf("%v: %v%v: %v\n",
					diag.DefaultSinkErrorPrefix, diag.DefaultSinkIDPrefix, d.ID, fmt.Sprintf(d.Message, bad)),
				sink.ErrorMsgs()[msg])
			msg++
		}
	}
}
