resource "simple_provider" "pulumi:providers:simple" {
}

default_provider "simple_provider" {
    resource "non_default_resource" "simple:index:Resource" {
        value = true
    }
}

resource "default_resource" "simple:index:Resource" {
    value = true
}
