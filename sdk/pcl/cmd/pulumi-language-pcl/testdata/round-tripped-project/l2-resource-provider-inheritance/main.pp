resource "provider" "pulumi:providers:simple" {}

resource "parent1" "simple:index:Resource" {
    value = true
    options {
        provider = provider
    }
}

// This should inherit the explicit provider from parent1
resource "child1" "simple:index:Resource" {
    value = true
    options {
        parent = parent1
    }
}


resource "parent2" "primitive:index:Resource" {
    boolean = false
    float = 0
    integer = 0
    string = ""
    numberArray = []
    booleanMap = {}
}

// This _should not_ inherit the provider from parent2 as it is a default provider.
resource "child2" "simple:index:Resource" {
    value = true
    options {
        parent = parent2
    }
}

// This _should not_ inherit the provider from parent1 as its from the wrong package.
resource "child3" "primitive:index:Resource" {
    boolean = false
    float = 0
    integer = 0
    string = ""
    numberArray = []
    booleanMap = {}
    options {
        parent = parent1
    }
}