resource "pet" "random:index:RandomPet" {
    options {
        version = "4.19.1"
    }
}

output "name" {
    value = pet.id
}
