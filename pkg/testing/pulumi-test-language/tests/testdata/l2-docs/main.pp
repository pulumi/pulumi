resource "res" "docs:index:Resource" {
    in = invoke("docs:index:fun", { in: false }).out
}