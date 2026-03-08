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
