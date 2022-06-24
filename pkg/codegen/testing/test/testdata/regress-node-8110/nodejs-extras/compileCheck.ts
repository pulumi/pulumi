import * as e from "./exampleFunc";
import { MyEnum } from "./types/enums";

// Check that common scenarios type-check.
export function checkCall() {
    e.exampleFunc({
        "enums": ["a", "b", "c"],
    });
    e.exampleFunc({
        "enums": [MyEnum.One, MyEnum.Two],
    });
    e.exampleFunc({
        "enums": ["a", "b", "c", MyEnum.One, MyEnum.Two],
    });
}
