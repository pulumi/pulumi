// Exercises per-value dependency propagation through an OutputValue-aware invoke. `pick:index:pickSecond`
// returns its `second` argument unchanged, so d.text semantically depends only on b. When the engine and
// provider both negotiate OutputValues on Invoke that is what state records; when the feature is disabled
// on the engine the SDK falls back to the union rule and d also gains a dependency on a.

resource "a" "simple-invoke:index:StringResource" {
    text = "a"
}

resource "b" "simple-invoke:index:StringResource" {
    text = "b"
}

data = invoke("pick:index:pickSecond", {
    first  = a.text
    second = b.text
})

resource "d" "simple-invoke:index:StringResource" {
    text = data.result
}

// `first` is secret, `second` is plain. In the OutputValues-aware path PickProvider returns the plain
// `second` verbatim, so e.text records no secret. In the legacy path the SDK unions
// the arg secretness onto the whole return, so e.text ends up secret.
first_secret = invoke("pick:index:pickSecond", {
    first  = secret(a.text)
    second = b.text
})

resource "e" "simple-invoke:index:StringResource" {
    text = first_secret.result
}

// `second` is itself secret, so the returned `second` carries that secret in both modes and f.text is
// secret regardless of the negotiation.
second_secret = invoke("pick:index:pickSecond", {
    first  = a.text
    second = secret(b.text)
})

resource "f" "simple-invoke:index:StringResource" {
    text = second_secret.result
}
