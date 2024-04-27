resource "failing" "fail_on_create:index:Resource" {
    value = false
}

resource "dependent" "simple:index:Resource" {
    value = true
    options {
        dependsOn = [failing]
    }
}

resource "dependent_on_output" "simple:index:Resource" {
    value = failing.value
}

resource "independent" "simple:index:Resource" {
    value = true
}

resource "double_dependency" "simple:index:Resource" {
    value = true
    options {
        dependsOn = [independent, dependent_on_output]
    }
}
