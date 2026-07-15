config text string {}

resource "prov" "pulumi:providers:config" {
    name = "my config"
}

resource "res" "config:index:Resource" {
    options {
        provider = prov
    }
    text = text
}

output "result" {
    value = res.text
}
