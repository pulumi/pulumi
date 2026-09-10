component "myComponent" "./providerComponent" {
    text = "hello"
}

output "result" {
    value = myComponent.result
}
