config hookTestFile "string" {}

hook "retryHook" {
    command = ["touch", hookTestFile]
}

resource "res" "flaky:index:FlakyCreate" {
    options {
        hooks = {
            onError = [retryHook]
        }
    }
}
