resource "firstResource" "camelNames:CoolModule:SomeResource" {
    theInput = true
}

resource "secondResource" "camelNames:CoolModule:SomeResource" {
    theInput = firstResource.theOutput
}
