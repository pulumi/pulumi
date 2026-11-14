// The resource names, the resource token and the property names are not identifiers. Property names are quoted.
// At the top level of a resource body, they go under `_`.
resource "first" "quoted-names:_my-module.v2/sub_3:-Resource_v1.x/thing-2" {
    __logicalName = "first res.v1/a"
    plain = "plain"
    _ = {
        "@top!" = {
            "@in.ner/x" = "inner"
        }
        "a.b/c" = "top"
    }
}

// A second resource reads the quoted outputs of the first. A property path names a quoted property as `_["..."]`.
resource "second" "quoted-names:_my-module.v2/sub_3:-Resource_v1.x/thing-2" {
    __logicalName = "second res.v1/b"
    plain = first.plain
    _ = {
        "@top!" = {
            "@in.ner/x" = first["@top!"]["@in.ner/x"]
        }
        "a.b/c" = first["a.b/c"]
    }
    options {
        ignoreChanges = [_["a.b/c"], _["@top!"]["@in.ner/x"]]
    }
}

// Function arguments are an object literal, so their names are quoted at every level.
invoked = invoke("quoted-names:_my-module.v2/sub_3:-invoke_v1.x/thing-2", {
    plain = first.plain
    "@top!" = {
        "@in.ner/x" = first["@top!"]["@in.ner/x"]
    }
    "a.b/c" = first["a.b/c"]
})

output "invoked" {
    value = invoked
}

output "invokedInner" {
    value = invoked["@top!"]["@in.ner/x"]
}
