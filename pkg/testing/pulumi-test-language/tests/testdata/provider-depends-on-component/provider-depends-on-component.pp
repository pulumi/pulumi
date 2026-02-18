// Component with no dependencies (the contrast)
resource "noDependsOn" "conformance-component:index:Simple" {
    value = true
}

// Component with dependsOn
resource "withDependsOn" "conformance-component:index:Simple" {
    value = true
    options {
        dependsOn = [noDependsOn]
    }
}

// Make a simple resource so that plugin detection works.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
