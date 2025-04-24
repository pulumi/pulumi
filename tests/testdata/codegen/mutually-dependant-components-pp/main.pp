component "first" "./first" {
    firstInput = second.data
}

component "second" "./second" {
    secondInput = first.data
}