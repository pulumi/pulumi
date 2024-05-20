resource "simple_provider" "pulumi:providers:simple" {
}

context {
    default_providers = [ "simple_provider" ]
    block {
        resource "non_default_resource" "simple:index:Resource" {
            value = true
        }
    }
}

resource "default_resource" "simple:index:Resource" {
    value = true
}
