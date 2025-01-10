# Since the name is "this" it will fail in typescript and other languages with
# this reservered keyword if it is not renamed.
resource "class" "simple:index:Resource" {
  value = true
}

resource "export" "simple:index:Resource" {
  value = true
}

resource "mod" "simple:index:Resource" {
  value = true
}

resource "import" "simple:index:Resource" {
  value = true
}

resource "object" "simple:index:Resource" {
  value = true
}

resource "self" "simple:index:Resource" {
  value = true
}

resource "this" "simple:index:Resource" {
  value = true
}

