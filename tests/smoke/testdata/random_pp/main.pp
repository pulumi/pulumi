resource "pet" "random:index:RandomPet" {
    options {
        version = "4.13.0"
    }
}

output "name" {
    value = pet.id
}