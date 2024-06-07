package codebase

import (
	"os"
	"testing"
)

func TestCodebase(t *testing.T) {
	c := NewCodebase()

	s3 := c.Module("aws/s3")

	s3.Interface(
		[]Modifier{Export},
		"I1",
		[]TypeParameter{},
	).Method(
		[]Modifier{},
		"m1",
		[]Argument{
			{Name: "p1", Type: StringT},
		},
		PromiseT(UnionT(NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT, NumberT)),
	)

	c1 := s3.Class(
		[]Modifier{Export, Abstract},
		"C1",
		[]TypeParameter{},
	)

	c1.Method(
		[]Modifier{Async},
		"m1",
		[]Argument{
			{Name: "p1", Type: StringT},
		},
		PromiseT(NumberT),
		[]Statement{
			ReturnS(IntE(42)),
		},
	).Documented(TD("").Parameter("p1", "The first parameter"))

	c1.Method(
		[]Modifier{},
		"m2",
		[]Argument{
			{Name: "p1", Type: StringT},
		},
		StringT,
		[]Statement{
			ReturnS(StringE("hello")),
		},
	).Documented(TD("Foo"))

	s3.Class(
		[]Modifier{Export, Abstract},
		"C2",
		[]TypeParameter{
			TP("T").Extends(ObjectT(map[string]Type{
				"a": StringT,
			})),
			TP("U").Extends(
				ObjectT(map[string]Type{
					"foo":  StringT,
					"bar":  NumberT,
					"baz":  UnionT(StringT, NumberT),
					"quux": RecordT(StringT, NumberT),
					"frob": UnionT(ArrayT(UnknownT)),
				}),
			),
			TP("V").Extends(
				UnionT(
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
					StringT,
				),
			),
		},
	).
		Extends(c1.AsType()).
		Method(
			[]Modifier{},
			"m4",
			[]Argument{
				{Name: "p1", Type: StringT},
			},
			NumberT,
			[]Statement{
				ConstS("x", IntE(53)),
				ConstTS("y", NullableT(RecordT(StringT, NumberT)), NullE),
				ConstS("z", MultiplyE(RefE("x"), AddE(RefE("x"), RefE("x")))),
				ReturnS(IntE(42)),
			},
		)

	i1 := s3.Interface(
		[]Modifier{Export},
		"I1",
		[]TypeParameter{},
	)

	s3.Interface(
		[]Modifier{Export},
		"I2",
		[]TypeParameter{
			TP("F").Extends(StringT),
		},
	).Extends(i1.AsType())

	files := c.Instantiate()
	for name, bs := range files {
		t.Logf("File: %s\n", name)
		t.Logf("Content: \n\n%s\n\n", string(bs))
		os.WriteFile("/tmp/tsc/test.ts", bs, 0o644)
	}
}
