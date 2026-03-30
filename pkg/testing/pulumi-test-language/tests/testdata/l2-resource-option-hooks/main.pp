config hookTestFile "string" {}
config hookPreviewFile "string" {}

hook "createHook" {
    command = ["touch", hookTestFile]
}

hook "previewHook" {
    command = ["touch", "${hookPreviewFile}_${args.name}"]
    onDryRun = true
}

resource "res" "simple:index:Resource" {
    value = true
    options {
        hooks = {
            beforeCreate = [createHook, previewHook]
        }
    }
}
