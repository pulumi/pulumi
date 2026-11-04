// Baseline for invoke dependency propagation: an invoke that reads properties from two different
// resources produces a return value whose consumer must depend on the union of both.

resource "a" "simple-invoke:index:StringResource" {
    text = "hello"
}

resource "b" "simple:index:Resource" {
    value = true
}

data = invoke("simple-invoke:index:secretInvoke", {
    value = a.text
    secretResponse = b.value
})

resource "d" "simple-invoke:index:StringResource" {
    text = data.response
}
