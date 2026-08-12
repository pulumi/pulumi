resource "component1" "component:index:ComponentCustomRefOutput" {
    value = "foo-bar-baz"
}

resource "component2" "component:index:ComponentCustomRefInputOutput" {
    inputRef = component1.ref
}

resource "custom1" "component:index:Custom" {
    value = component2.inputRef.value
}

resource "custom2" "component:index:Custom" {
    value = component2.outputRef.value
}
