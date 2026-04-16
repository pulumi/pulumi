# first & second are simple mutually dependent components

component "first" "./first" {
    input = second.untainted
}

component "second" "./second" {
    input = first.untainted
}

# another & many are also mutually dependent components, but many tests that the mutual dependency works through
# `range`.

component "another" "./first" {
    # We do the join + for + == dance because we want to force a value that depends on the contents of the list, not
    # just it's length (which may be known at preview time).
    input = (join("", [ for _, v in many : v.untainted ? "a" : "b" ]) == "xyz")
}

component "many" "./second" {
    options {
        range = 2
    }
    input = another.untainted
}
