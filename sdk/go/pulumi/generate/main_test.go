package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		give string
		want string
	}{
		{"foo-bar.go.template", "foo/bar.go"},
		{"bar.go.template", "bar.go"},
		{"fizz-buz-bar.go.template", "fizz/buz/bar.go"},
		{"foo-bar.tmpl.template", "foo/bar.tmpl"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			t.Parallel()

			got := templateFilePath(tt.give)
			assert.Equal(t, tt.want, got)
		})
	}
}
