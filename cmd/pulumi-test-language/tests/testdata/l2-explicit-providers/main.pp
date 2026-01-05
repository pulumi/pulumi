resource "explicit" "pulumi:providers:component" {
}

resource "list" "component:index:ComponentCallable" {
    options {
        providers = [explicit]
    }

    value = "value"
}

resource "map" "component:index:ComponentCallable" {
    options {
        providers = {
            component = explicit
        }
    }

    value = "value"
}
