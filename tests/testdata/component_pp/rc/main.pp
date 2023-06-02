config "prefix" "string" {

}

resource "pet" "random:index:RandomPet" {
    options {
        version = "4.13.0"
    }

    prefix = prefix
}

output "name" {
    value = pet.id
}