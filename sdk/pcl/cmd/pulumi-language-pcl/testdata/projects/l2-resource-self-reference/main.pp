resource "root" "selfref:index:Node" {
}

resource "child" "selfref:index:Node" {
    parent = root
    parents = [root]
    namedParents = { root = root }
    parentOrName = root
}
