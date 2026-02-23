resource "simpleV2" "pulumi:providers:simple" {
}

resource "withV22" "conformance-component:index:Simple" {
    value = true
    options {
        version = "22.0.0"
        providers = {
            simple = simpleV2
        }
    }
}

resource "withDefault" "conformance-component:index:Simple" {
    value = true
    options {
        providers = {
            simple = simpleV2
        }
    }
}
