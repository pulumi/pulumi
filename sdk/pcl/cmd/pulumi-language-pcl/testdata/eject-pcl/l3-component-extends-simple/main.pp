resource "derived" "inherit:index:Derived" {
    message = "hello"
    scale = 3
}

output "baseOutput" {
    value = derived.baseOutput
}

output "derivedOutput" {
    value = derived.derivedOutput
}
