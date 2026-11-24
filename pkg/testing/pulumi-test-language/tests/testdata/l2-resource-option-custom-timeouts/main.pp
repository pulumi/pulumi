config createTimeout "string" {}

resource "noTimeouts" "simple:index:Resource" {
    value = true
}

resource "createOnly" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            create = "5m"
        }
    }
}

resource "updateOnly" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            update = "10m"
        }
    }
}

resource "deleteOnly" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            delete = "3m"
        }
    }
}

resource "readOnly" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            read = "9m"
        }
    }
}

resource "allTimeouts" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            create = "2m"
            update = "4m"
            delete = "1m"
            read = "5m"
        }
    }
}

resource "configTimeout" "simple:index:Resource" {
    value = true
    options {
        customTimeouts = {
            create = createTimeout
        }
    }
}
