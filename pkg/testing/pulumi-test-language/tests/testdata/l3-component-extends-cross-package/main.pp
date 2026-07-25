resource "derived" "inheritderived:index:DerivedComponent" {
    message = "hello"
    scale = 7
}

output "baseOutput" {
    value = derived.baseOutput
}

output "derivedOutput" {
    value = derived.derivedOutput
}
