resource "provider" "pulumi:providers:simple-invoke" {}

resource "parent" "simple-invoke:index:StringResource" {
    text = "parent"
    options {
        provider = provider
    }
}

// This should inherit the explicit provider from parent.
resource "child" "simple-invoke:index:StringResource" {
    text = "child"
    options {
        parent = parent
    }
}

// This should use the explicit provider of parent.
parentData = invoke("simple-invoke:index:myInvoke", { value = "parent" }, {
    parent = parent
})

// This should use the explicit provider that child inherited from parent.
childData = invoke("simple-invoke:index:myInvoke", { value = "child" }, {
    parent = child
})

output "parentResult" {
    value = parentData.result
}

output "childResult" {
    value = childData.result
}
