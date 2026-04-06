resource "myComponent" "plaincomponent:index:Component" {
    name = "my-resource"
    settings = {
        enabled = true
        tags = {
            "env" = "test"
        }
    }
}

output "label" {
    value = myComponent.label
}
