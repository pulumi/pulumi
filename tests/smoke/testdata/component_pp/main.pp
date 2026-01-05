component "myPet" "./rc" {
    prefix = "my-"
}

output "name" {
    value = myPet.name
}