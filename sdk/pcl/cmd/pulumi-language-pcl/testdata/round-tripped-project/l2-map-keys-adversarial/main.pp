resource "res" "primitive:index:Resource" {
    boolean = false
    float = 2.17
    integer = -12
    string = "adversarial"
    numberArray = [0, 1]
    booleanMap = {
        "__type": true,
        "__internal": false,
        "__provider": true,
        "__version": false
        "" = true,
        "Some $${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)" = false,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash" = true,
    }
}

invokeResult = invoke("primitive:index:invoke", {
    boolean = false
    float = 2.17
    integer = -12
    string = "adversarial"
    numberArray = [0, 1]
    booleanMap = {
        "__type": true,
        "__internal": false,
        "__provider": true,
        "__version": false
        "" = true,
        "Some $${common} \"characters\" 'that' need escaping: \\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)" = false,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash" = true,
    }
})

output "resourceBooleanMap" {
    value = res.booleanMap
}

output "invokeBooleanMap" {
    value = invokeResult.booleanMap
}
