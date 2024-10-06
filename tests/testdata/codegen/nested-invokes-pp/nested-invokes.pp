example = invoke("std:index:replace", {
    text = invoke("std:index:upper", { input = "hello_world" }).result
    search = "_"
    replace = "-"
})