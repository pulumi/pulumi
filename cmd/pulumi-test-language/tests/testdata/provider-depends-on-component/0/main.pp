resource "noDependsOn" "simple:index:Resource" {
    value = true
}

resource "component" "conformance-component:index:Simple" {
    value = true
    options {
        dependsOn = [noDependsOn]
    }
}

resource "simpleResource" "simple:index:Resource" {
    value = false
}
