resource "componentRes" "component:index:ComponentCustomRefOutput" {
    value = "foo-bar-baz"
}

resource "res" "namespaced:index:Resource" {
    value = true
    resourceRef = componentRes.ref
}
