component "first" "./first" {
    passwordLength = second.passwordLength
}

component "second" "./second" {
    petName = first.petName
}