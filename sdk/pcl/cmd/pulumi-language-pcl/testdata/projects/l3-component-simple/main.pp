resource "input" "simple:index:Resource" {
    value = true
}

component someComponent "./myComponent" {
    input = input.value
}

output result { 
    value = someComponent.output
}
