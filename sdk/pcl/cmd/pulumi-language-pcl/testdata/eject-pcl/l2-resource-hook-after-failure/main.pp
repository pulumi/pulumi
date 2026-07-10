hook resource "failingHook" {
    command = ["false"]
}

resource "res" "simple:index:Resource" {
    value = true
    options {
        hooks = {
            afterCreate = [failingHook]
        }
    }
}
