config hookTestFile "string" {}

hook error "retryHook" {
    command = ["touch", hookTestFile]
}

resource "res" "flaky:index:FlakyCreate" {
    options {
        hooks = {
            onError = [retryHook]
        }
    }
}
