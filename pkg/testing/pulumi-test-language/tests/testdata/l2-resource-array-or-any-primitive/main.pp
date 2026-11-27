resource "stringList" "array-or-any:index:StringList" {
    values = ["a", "b", "c"]
}

resource "stringAny" "array-or-any:index:StringList" {
    values = {
        any = true
    }
}
