resource "noTimeouts" "conformance-component:index:Simple" {
    value = true
}

resource "createOnly" "conformance-component:index:Simple" {
    value = true
    options {
        customTimeouts = {
            create = "5m"
        }
    }
}

resource "updateOnly" "conformance-component:index:Simple" {
    value = true
    options {
        customTimeouts = {
            update = "10m"
        }
    }
}

resource "deleteOnly" "conformance-component:index:Simple" {
    value = true
    options {
        customTimeouts = {
            delete = "3m"
        }
    }
}

resource "allTimeouts" "conformance-component:index:Simple" {
    value = true
    options {
        customTimeouts = {
            create = "2m"
            update = "4m"
            delete = "1m"
        }
    }
}

// Ensure the simple plugin is discoverable for this conformance run.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
