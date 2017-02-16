// This tests the code-generated expansion of a switch statement.

function sw(v: string): string {
    let result: string = "";
    switch (v) {
        case "a":
            result += "a";
            break;
        case "b":
            result += "b";
        default:
            result += "d";
            break;
    }
    return result;
}

let a = sw("a");
if (a !== "a") {
    throw new Error("Expected 'a'");
}

let b = sw("b");
if (b !== "bd") {
    throw new Error("Expected 'bd'");
}

let d = sw("d");
if (d !== "d") {
    throw new Error("Expected 'd'");
}

