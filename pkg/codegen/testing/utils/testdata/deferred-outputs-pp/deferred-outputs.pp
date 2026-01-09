component "first" "./first" {
    passwordLength = second.passwordLength
}

component "second" "./second" {
    petName = first.petName
}

component "another" "./first" {
    passwordLength = length([ for _, v in many : v.passwordLength ])
}

component "many" "./second" {
    options { range = 10 }
    petName = another.petName
}