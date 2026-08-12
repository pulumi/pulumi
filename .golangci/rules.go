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

// ptrHelperName forbids private pointer-wrapper helpers whose body is just
// `return &v` from being named anything other than `ptr`. It catches both the
// monomorphic form (e.g. `intPtr`, `strPtr`, `continuationPtr`) and the generic
// form (e.g. `func ref[T any](v T) *T { return &v }`). This avoids unnecessary
// duplication.
func ptrHelperName(m dsl.Matcher) {
	m.Match(
		`func $name($v $T) *$T { return &$v }`,
		`func $name[$T $_]($v $T) *$T { return &$v }`,
	).
		Where(m["name"].Text != "ptr" && m["name"].Text.Matches(`^[a-z]`)).
		Report(`pointer-wrapping helper "$name" must be named "ptr"; prefer a single generic func ptr[T any](v T) *T { return &v }`)
}
