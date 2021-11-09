package parsing

import (
	"testing"
)

func TestJsonParsing(t *testing.T) {
	s := `{ "jo" : 89, "phinues": "pherb"}`
	ParseJSON(s)
	t.FailNow()
}
