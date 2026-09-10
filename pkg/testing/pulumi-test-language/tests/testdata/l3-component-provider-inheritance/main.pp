resource "explicit" "pulumi:providers:simple" {}

component "withProviders" "./local" {
    options {
        providers = {
            simple = explicit
        }
    }
}

output "result" {
    value = withProviders.result
}
