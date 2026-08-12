resource "pet" "random:index:RandomPet" {
    options {
        version = "4.19.0"
    }

    length = aVariable
}

output "name" {
    value = pet.id
}
