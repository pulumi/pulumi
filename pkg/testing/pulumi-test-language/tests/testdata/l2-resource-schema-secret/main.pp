resource "prov" "pulumi:providers:output" {
    elideUnknowns = true
}

resource "topLevel" "output:index:Resource" {
    value = 1
    options {
        provider = prov
    }
}

resource "nested" "output:index:ComplexResource" {
    value = 1
    options {
        provider = prov
    }
}

output "topLevel" {
    value = topLevel.secretOutput
}

output "nested" {
    value = nested.outputObject.secretOutput
}
