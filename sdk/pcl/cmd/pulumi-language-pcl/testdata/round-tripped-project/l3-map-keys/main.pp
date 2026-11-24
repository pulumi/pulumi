component "cmp" "./myComponent" {
    booleanMap = {
        "my key" = false,
        "my.key" = true,
        "my-key" = false,
        "my_key" = true,
        "MY_KEY" = false,
        "myKey" = true,
        "__type": true,
        "__internal": false,
        "__provider": true,
        "__version": false
        "" = true,
        "Some $${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)" = false,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash" = true,
    }
}

output "resourceBooleanMap" {
    value = cmp.booleanMap
}
