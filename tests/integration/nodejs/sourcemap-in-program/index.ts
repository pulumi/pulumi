export function willThrow() {
    if (true) {
        throw new Error("this is a test error");
    }
}

willThrow();
