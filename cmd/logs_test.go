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
	assert.Equal(t, "2006-01-02T15:04:05-08:00", c.Format(time.RFC3339))

	d, _ := parseSince("2006-01-02")
	assert.Equal(t, "2006-01-02T00:00:00-08:00", d.Format(time.RFC3339))
}
