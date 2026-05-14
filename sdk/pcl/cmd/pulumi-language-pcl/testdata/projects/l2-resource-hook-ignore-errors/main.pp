hook "failingHook" {
    command      = ["false"]
    ignoreErrors = true
}

resource "res" "simple:index:Resource" {
    value = true
    options {
        hooks = {
            afterCreate = [failingHook]
        }
    }
}
