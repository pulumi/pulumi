config hookTestFile "string" {}
config hookPreviewFile "string" {}

hook resource "createHook" {
    command = ["touch", hookTestFile]
}

hook resource "previewHook" {
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
