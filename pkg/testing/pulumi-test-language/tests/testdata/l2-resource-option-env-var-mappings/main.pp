resource "prov" "pulumi:providers:simple" {
    options {
        envVarMappings = {
            "MY_VAR" = "PROVIDER_VAR"
            "OTHER_VAR" = "TARGET_VAR"
        }
    }
}
