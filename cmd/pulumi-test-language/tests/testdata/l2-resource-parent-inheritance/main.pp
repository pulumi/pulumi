resource "provider" "pulumi:providers:simple" {}

resource "parent1" "simple:index:Resource" {
    value = true
    options {
        provider = provider
    }
}

resource "child1" "simple:index:Resource" {
    value = true
    options {
        parent = parent1
    }
}

resource "orphan1" "simple:index:Resource" {
    value = true
}

resource "parent2" "simple:index:Resource" {
    value = true
    options {
        protect = true
    }
}

resource "child2" "simple:index:Resource" {
    value = true
    options {
        parent = parent2
    }
}

resource "child3" "simple:index:Resource" {
    value = true
    options {
        parent = parent2
        protect = false
    }
}

resource "orphan2" "simple:index:Resource" {
    value = true
}
