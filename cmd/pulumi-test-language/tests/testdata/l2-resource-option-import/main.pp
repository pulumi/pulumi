resource "import" "simple:index:Resource" {
    value = true
    options {
        import = "fakeID123"
    }
}

resource "notImport" "simple:index:Resource" {
    value = true
}
