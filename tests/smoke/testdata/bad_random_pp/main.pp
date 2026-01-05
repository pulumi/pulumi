resource "pet" "random:index:RandomPet" {
    options {
        version = "4.13.0"
    }

    length = aVariable
}

output "name" {
    value = pet.id
}