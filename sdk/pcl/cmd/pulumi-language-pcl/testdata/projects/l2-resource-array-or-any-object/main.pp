resource "objectList" "array-or-any:index:ObjectList" {
    values = [
        { itemName = "first" },
        { itemName = "second" },
    ]
}

resource "objectAny" "array-or-any:index:ObjectList" {
    values = {
        any = true
    }
}
