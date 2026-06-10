resource "first" "manifest:index:Resource" {
    kind = "Manifest"
    metadata = {
        "name" = "first"
        "labels" = {
            "app" = "first"
        }
    }
    spec = {
        "replicas" = 1
        "template" = {
            "metadata" = {
                "name" = "inner"
            }
            "containers" = [{
                "name"  = "app"
                "image" = "nginx"
                "ports" = [80]
            }]
        }
    }
}

// `kind` has a constant value in the schema; reading it must bind without type errors.
output "kind" {
    value = first.kind
}
