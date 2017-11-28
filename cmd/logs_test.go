// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseSince(t *testing.T) {
	a, _ := parseSince("")
	assert.Nil(t, a)

	now := time.Now()
	b, _ := parseSince("1m30s")
	assert.True(t, b.UnixNano() < now.UnixNano())
	fmt.Printf("Res: %v\n", b)

	c, _ := parseSince("2006-01-02T15:04:05")
	assert.Equal(t, "Mon Jan  2 15:04:05 2006", c.Format(time.ANSIC))

	d, _ := parseSince("2006-01-02")
	assert.Equal(t, "Mon Jan  2 00:00:00 2006", d.Format(time.ANSIC))
}
