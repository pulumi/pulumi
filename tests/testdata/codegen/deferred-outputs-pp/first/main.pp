config "passwordLength" "number" { }

resource "randomPet" "random:index/randomPet:RandomPet" {

}

resource "randomPassword" "random:index/randomPassword:RandomPassword" {
    length = passwordLength
}

output "petName" {
    value = randomPet.id
}

output "password" {
    value = randomPassword.result
}
