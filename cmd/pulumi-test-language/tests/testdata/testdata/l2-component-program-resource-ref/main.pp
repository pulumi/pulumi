resource "component1" "component:index:ComponentCustomRefOutput" {
    value = "foo-bar-baz"
}

resource "custom1" "component:index:Custom" {
    value = component1.value
}

resource "custom2" "component:index:Custom" {
    value = component1.ref.value
}
