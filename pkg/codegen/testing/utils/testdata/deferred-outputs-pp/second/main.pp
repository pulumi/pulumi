config "petName" "string" { }

resource "randomPet" "random:index/randomPet:RandomPet" {
    length = length(petName)
}

resource "password" "random:index/randomPassword:RandomPassword" {
    length = 16
    special = true
    numeric = false
}

output "passwordLength" {
    value = password.length
}