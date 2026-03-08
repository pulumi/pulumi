output "secret" {
    value = invoke("scalar-returns:index:invokeSecret", {value="goodbye"})
}

output "array" {
    value = invoke("scalar-returns:index:invokeArray", {value="the word"})
}

output "map" {
    value = invoke("scalar-returns:index:invokeMap", {value="hello"})
}

output "secretMap" {
    value = invoke("scalar-returns:index:invokeMap", {value="secret"})
}