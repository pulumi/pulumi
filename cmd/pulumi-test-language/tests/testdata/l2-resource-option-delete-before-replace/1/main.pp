// Stage 1: Change properties to trigger replacements

// Resource with deleteBeforeReplace option - should delete before creating
resource "withOption" "simple:index:Resource" {
    value = false  // Changed to trigger replacement
    options {
        replaceOnChanges = [value]
        deleteBeforeReplace = true
    }
}

// Resource without deleteBeforeReplace - should create before deleting (default)
resource "withoutOption" "simple:index:Resource" {
    value = false  // Changed to trigger replacement
    options {
        replaceOnChanges = [value]
    }
}
