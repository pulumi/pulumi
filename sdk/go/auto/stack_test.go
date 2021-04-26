package auto

import (
	"fmt"
	"testing"
)

const testPermalink = "Permalink: https://gotest"

func TestGetPermalink(t *testing.T) {
	tests := map[string]struct {
		testee string
		want   string
		err    error
	}{
		"successful parsing": {testee: fmt.Sprintf("%s\n", testPermalink), want: "https://gotest"},
		"failed parsing":     {testee: testPermalink, err: ErrParsePermalinkFailed},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := GetPermalink(test.testee)
			if err != nil {
				if test.err == nil || test.err != err {
					t.Errorf("got '%v', want '%v'", err, test.err)
				}
			}

			if got != test.want {
				t.Errorf("got '%s', want '%s'", got, test.want)
			}
		})
	}

}
