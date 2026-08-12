hook resource "panicHook" {
    command = [notImplemented("hook panic")]
}

resource "res" "simple:index:Resource" {
    value = true
    options {
        hooks = {
            afterCreate = [panicHook]
        }
    }
}
