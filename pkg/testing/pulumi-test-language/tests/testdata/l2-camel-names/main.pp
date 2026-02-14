resource "firstResource" "camelNames:CoolModule:SomeResource" {
    theInput = true
}

resource "secondResource" "camelNames:CoolModule:SomeResource" {
    theInput = firstResource.theOutput
}

resource "thirdResource" "camelNames:CoolModule:SomeResource" {
    theInput = true
    resourceName = "my-cluster"
}
