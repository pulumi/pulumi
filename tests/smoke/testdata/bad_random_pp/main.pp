resource "pet" "random:index:RandomPet" {
    options {
        version = "4.19.1"
    }

    length = aVariable
}

output "name" {
    value = pet.id
}
