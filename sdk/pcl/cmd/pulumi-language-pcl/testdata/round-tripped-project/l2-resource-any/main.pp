resource "aString" "any-handled:index:Resource" {
    value = "a string"
}

resource "aBoolean" "any-handled:index:Resource" {
    value = true
}

resource "aNumber" "any-handled:index:Resource" {
    value = 42
}

resource "aList" "any-handled:index:Resource" {
    value = [1, true, "three"]
}

resource "anObject" "any-handled:index:Resource" {
    value = {
        key    = "value"
        nested = { count = 1 }
    }
}

resource "anAsset" "any-handled:index:Resource" {
    value = stringAsset("the asset contents")
}
