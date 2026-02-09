// Make a simple resource to use as a parent
resource "parent" "simple:index:Resource" {
    value = true
}

resource "aliasURN" "simple:index:Resource" {
    value = true
}

resource "aliasName" "simple:index:Resource" {
    value = true
}

resource "aliasNoParent" "simple:index:Resource" { 
    value = true
}

resource "aliasParent" "simple:index:Resource" { 
    value = true
    options {
        parent = aliasURN
    }
}
