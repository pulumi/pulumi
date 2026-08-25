package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

func ignoreErrorClose(m dsl.Matcher) {
	m.Match(`contract.IgnoreError($x.Close())`).
		Report("use contract.IgnoreClose($x) instead of contract.IgnoreError($x.Close())").
		Suggest("contract.IgnoreClose($x)")
}

func deferIgnoreClose(m dsl.Matcher) {
	m.Match(`defer func() { contract.IgnoreClose($x) }()`).
		Report("use defer contract.IgnoreClose($x) directly instead of wrapping in func literal").
		Suggest("defer contract.IgnoreClose($x)")
}

func errorsAs(m dsl.Matcher) {
	m.Match(`errors.As($err, $target)`).
		Report("use errors.AsType[T] instead of errors.As")
}

// ptrHelper forbids private pointer-wrapper helpers whose body is just
// `return &v`. Use Go 1.26's `new(expr)` instead of adding a helper like
// `func ptr[T any](v T) *T { return &v }`.
func ptrHelper(m dsl.Matcher) {
	m.Match(
		`func $name($v $T) *$T { return &$v }`,
		`func $name[$T any]($v $T) *$T { return &$v }`,
		`$name := func($v $T) *$T { return &$v }`,
	).
		Where(m["name"].Text.Matches(`^[a-z]`)).
		Report(`pointer-wrapping helper "$name" is unnecessary; use new(expr) instead`)
}
