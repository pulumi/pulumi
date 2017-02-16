// Test out the various kinds of loops.

function loops() {
    let x = 0;
    while (x < 10) {
        x++;
    }
    if (x != 10) {
        throw "Expected x == 10";
    }

    let last = false;
    for (let i = 0; i < 10; i++) {
        if (i == 9) {
            last = true;
        }
    }
    if (!last) {
        throw "Expected last == true";
    }
}

