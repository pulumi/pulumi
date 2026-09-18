resource "src" "simple:index:Resource" {
    value = true
}

read "res" "read:index:Resource" {
    id = "existing-id"
    lookup = "existing-key"
    options {
        dependsOn = [src]
    }
}
