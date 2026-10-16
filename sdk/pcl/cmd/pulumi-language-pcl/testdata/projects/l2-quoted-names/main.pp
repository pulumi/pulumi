// Property names that are not identifiers are quoted. At the top level of a resource body, they go under `_`.
resource "res" "quoted-names:index:Resource" {
    plain = "plain"
    _ = {
        "@top!" = {
            "@inner!" = "inner"
        }
        "$top.2" = "top"
    }
}

output "resourceInner" {
    value = res["@top!"]["@inner!"]
}

output "resourceTop" {
    value = res["$top.2"]
}

// Function arguments are an object literal, so their names are quoted at every level.
invoked = invoke("quoted-names:index:invoke", {
    plain = res.plain
    "@top!" = {
        "@inner!" = res["@top!"]["@inner!"]
    }
    "$top.2" = res["$top.2"]
})

output "invoked" {
    value = invoked
}

output "invokedInner" {
    value = invoked["@top!"]["@inner!"]
}
