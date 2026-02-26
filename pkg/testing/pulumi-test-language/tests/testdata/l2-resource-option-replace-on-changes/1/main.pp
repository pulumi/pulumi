// Stage 1: Change properties to trigger replacements

// Scenario 1: Change replaceProp → REPLACE (schema triggers)
resource "schemaReplace" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = false  // Changed from true
}

// Scenario 2: Change value → REPLACE (option triggers)
resource "optionReplace" "replaceonchanges:index:ResourceB" {
    value = false  // Changed from true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 3: Change value → REPLACE (option on value triggers)
resource "bothReplaceValue" "replaceonchanges:index:ResourceA" {
    value = false  // Changed from true
    replaceProp = true  // Unchanged
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 4: Change replaceProp → REPLACE (schema on replaceProp triggers)
resource "bothReplaceProp" "replaceonchanges:index:ResourceA" {
    value = true  // Unchanged
    replaceProp = false  // Changed from true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 5: Change value → UPDATE (no replaceOnChanges)
resource "regularUpdate" "replaceonchanges:index:ResourceB" {
    value = false  // Changed from true
}

// Scenario 6: No change → SAME (no operation)
resource "noChange" "replaceonchanges:index:ResourceB" {
    value = true  // Unchanged
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 7: Change replaceProp (not value) → UPDATE (marked property unchanged)
resource "wrongPropChange" "replaceonchanges:index:ResourceA" {
    value = true  // Unchanged (this is marked for replacement)
    replaceProp = false  // Changed from true (this is NOT marked for replacement by option)
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 8: Change value → REPLACE (multiple properties marked)
resource "multiplePropReplace" "replaceonchanges:index:ResourceA" {
    value = false  // Changed from true
    replaceProp = true  // Unchanged
    options {
        replaceOnChanges = [value, replaceProp]
    }
}

// Remote component: change value → REPLACE
resource "remoteWithReplace" "conformance-component:index:Simple" {
    value = false  // Changed from true
    options {
        replaceOnChanges = [value]
    }
}

// Keep a simple resource so all expected plugins are required.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
