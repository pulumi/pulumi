resource "pet" "random:index:RandomPet" {
    options {
        version = "4.19.0"
    }
}

output "name" {
    value = pet.id
}
