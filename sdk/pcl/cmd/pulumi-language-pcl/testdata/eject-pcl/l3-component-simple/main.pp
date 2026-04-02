component someComponent "./myComponent" {
    input = true
}

output result { 
    value = someComponent.output
}