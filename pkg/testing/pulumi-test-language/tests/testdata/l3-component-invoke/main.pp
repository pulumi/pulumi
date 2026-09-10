resource "prov" "pulumi:providers:config" {
    name = "my config"
}

component "myComponent" "./invokeComponent" {
    options {
        providers = [prov]
    }
}
