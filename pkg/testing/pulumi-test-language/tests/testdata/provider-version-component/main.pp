resource "simpleV2" "pulumi:providers:simple" {
}

resource "withV2" "conformance-component:index:Simple" {
    value = true
    options {
        version = "2.0.0"
        providers = {
            simple = simpleV2
        }
    }
}

resource "withV26" "conformance-component:index:Simple" {
    value = false
    options {
        version = "26.0.0"
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
