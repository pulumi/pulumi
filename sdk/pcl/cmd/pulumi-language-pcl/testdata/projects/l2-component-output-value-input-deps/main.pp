resource "a" "output-value-input-deps:index:Custom" {
    value = "a"
}

resource "b" "output-value-input-deps:index:Custom" {
    value = "b"
}

resource "comp" "output-value-input-deps:index:Component" {
    refs = {
        "x" = a.value,
        "y" = b.value,
    }
}
