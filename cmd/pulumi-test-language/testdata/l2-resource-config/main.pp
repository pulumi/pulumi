resource "prov" "pulumi:providers:config" {
    name = "my config"
    pluginDownloadURL = "not the same as the pulumi resource option"
}

// Note this isn't _using_ the explicit provider, it's just grabbing a value from it.
resource "res" "config:index:Resource" {
    text = prov.version
}

output "pluginDownloadURL" { 
    value = prov.pluginDownloadURL
}