package deepcopy

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepCopy(t *testing.T) {
	cases := []interface{}{
		bool(false),
		bool(true),
		int(-42),
		int8(-42),
		int16(-42),
		int32(-42),
		int64(-42),
		uint(42),
		uint8(42),
		uint16(42),
		uint32(42),
		uint64(42),
		float32(3.14159),
		float64(3.14159),
		complex64(complex(3.14159, -42)),
		complex(3.14159, -42),
		"foo",
		[2]byte{42, 24},
		[]byte{0, 1, 2, 3},
		[]string{"foo", "bar"},
		map[string]int{
			"a": 42,
			"b": 24,
		},
		struct {
			Foo int
			Bar map[int]int
		}{
			Foo: 42,
			Bar: map[int]int{
				19: 77,
			},
		},
		[]map[string]string{
			{
				"foo": "bar",
				"baz": "qux",
			},
			{
				"alpha": "beta",
			},
		},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			assert.EqualValues(t, c, Copy(c))
		})
	}
}
