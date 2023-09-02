resource "small" "long:index:Resource" {
    value = 256
}

resource "min53" "long:index:Resource" { 
    value = -9007199254740992
}

resource "max53" "long:index:Resource" { 
    value = 9007199254740992
}

resource "min64" "long:index:Resource" {
    value = -9223372036854775808
}

resource "max64" "long:index:Resource" {
    value = 9223372036854775807
}

resource "uint64" "long:index:Resource" {
    value = 18446744073709551615
}

resource "huge" "long:index:Resource" {
    value = 20000000000000000001
}

output "huge" { 
    value = 20000000000000000001
}

output "roundtrip" {
    value = huge.value
}

output "result" {
    value = small.value + min53.value + max53.value + min64.value + max64.value + uint64.value + huge.value
}