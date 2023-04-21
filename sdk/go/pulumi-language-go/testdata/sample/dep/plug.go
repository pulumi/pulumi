package dep

import indirect "example.com/indirect-dep/v2"

func Bar() string {
	indirect.Baz()
	return "bar"
}
