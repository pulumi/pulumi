// Stage 0: Initial resource creation

// Scenario 1: Schema-based replaceOnChanges on replaceProp
resource "schemaReplace" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = true
}

// Scenario 2: Option-based replaceOnChanges on value
resource "optionReplace" "replaceonchanges:index:ResourceB" {
    value = true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 3: Both schema and option - will change value
resource "bothReplaceValue" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 4: Both schema and option - will change replaceProp
resource "bothReplaceProp" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 5: No replaceOnChanges - baseline update
resource "regularUpdate" "replaceonchanges:index:ResourceB" {
    value = true
}

// Scenario 6: replaceOnChanges set but no change
resource "noChange" "replaceonchanges:index:ResourceB" {
    value = true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 7: replaceOnChanges on value, but only replaceProp changes
resource "wrongPropChange" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = true
    options {
        replaceOnChanges = [value]
    }
}

// Scenario 8: Multiple properties in replaceOnChanges array
resource "multiplePropReplace" "replaceonchanges:index:ResourceA" {
    value = true
    replaceProp = true
    options {
        replaceOnChanges = [value, replaceProp]
    }
}

// Remote component with replaceOnChanges
resource "remoteWithReplace" "component:index:ComponentCallable" {
    value = "one"
    options {
        replaceOnChanges = [value]
    }
}

// Keep a simple resource so all expected plugins are required.
resource "simpleResource" "simple:index:Resource" {
    value = false
}
