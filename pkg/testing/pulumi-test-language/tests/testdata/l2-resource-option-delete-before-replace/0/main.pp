// Stage 0: Initial resource creation

// Resource with deleteBeforeReplace option
resource "withOption" "simple:index:Resource" {
    value = true
    options {
        replaceOnChanges = [value]
        deleteBeforeReplace = true
    }
}

// Resource without deleteBeforeReplace (default create-before-delete behavior)
resource "withoutOption" "simple:index:Resource" {
    value = true
    options {
        replaceOnChanges = [value]
    }
}
