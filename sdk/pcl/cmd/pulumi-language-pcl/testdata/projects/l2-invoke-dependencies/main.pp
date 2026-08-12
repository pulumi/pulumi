resource "first" "simple:index:Resource" {
    value = false
}

// assert that resource second depends on resource first
// because it uses .secret from the invoke which depends on first
resource "second" "simple:index:Resource" {
    value = invoke("simple-invoke:index:secretInvoke", {
         value = "hello"
         secretResponse = first.value
    }).secret
}

resource "third" "simple-invoke:index:StringResource" {
    text = "third"
}

// third.text is known during preview, but third does not exist yet. SDKs must
// infer the dependency on third from the invoke's arguments and skip the
// invoke while third's ID is unknown: getText fails if it is called before
// third has been created.
data = invoke("simple-invoke:index:getText", {
    text = third.text
})

resource "fourth" "simple-invoke:index:StringResource" {
    text = data.result
}