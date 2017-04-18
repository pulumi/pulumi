// Test out the various kinds of loops.

// test some loops within a function.
function loops() {
    let x = 0;
    while (x < 10) {
        x++;
    }
    if (x !== 10) {
        throw "Expected x == 10";
    }

    let last = false;
    for (let i = 0; i < 10; i++) {
        if (i === 9) {
            last = true;
        }
    }
    if (!last) {
        throw "Expected last == true";
    }
}

// now test those same loops at the module's top-level.
let x = 0;
while (x < 10) {
    x++;
}
if (x !== 10) {
    throw "Expected x == 10";
}

let last = false;
for (let i = 0; i < 10; i++) {
    if (i === 9) {
        last = true;
    }
}
if (!last) {
    throw "Expected last == true";
}

