resource "x" "unionref:index:XType" {}

resource "y" "unionref:index:YType" {}

resource "z" "unionref:index:ZType" {
    value = x
    wrapper = { value = y }
    values = [x, y]
}
