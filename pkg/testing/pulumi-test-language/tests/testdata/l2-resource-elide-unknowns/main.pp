// This test checks that when a provider doesn't return properties for fields it considers unknown the runtime
// can still access that field as an output.

resource "prov" "pulumi:providers:output" {
    elideUnknowns = true
}

resource "unknown" "output:index:Resource" {
    value = 1
    options {
        provider = prov
    }
}

resource "complex" "output:index:ComplexResource" {
    value = 1
    options {
        provider = prov
    }
}

// Try and use the unknown output as an input to another resource to check that it doesn't cause any issues.
resource "res" "simple:index:Resource" {
    value = unknown.output == "hello"
}

resource "resArray" "simple:index:Resource" {
    value = complex.outputArray[0] == "hello"
}

resource "resMap" "simple:index:Resource" {
    value = complex.outputMap["x"] == "hello"
}

resource "resObject" "simple:index:Resource" {
    value = complex.outputObject.output == "hello"
}

// And try to use it has an output
output "out" {
    value = unknown.output
}

output "outArray" {
    value = complex.outputArray[0]
}

output "outMap" {
    value = complex.outputMap["x"]
}

output "outObject" {
    value = complex.outputObject.output
}
