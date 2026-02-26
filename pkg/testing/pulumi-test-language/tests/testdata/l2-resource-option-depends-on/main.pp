resource "noDependsOn" "simple:index:Resource" {
    value = true
}

resource "withDependsOn" "simple:index:Resource" {
    value = false
    options {
        dependsOn = [noDependsOn]
    }
}
