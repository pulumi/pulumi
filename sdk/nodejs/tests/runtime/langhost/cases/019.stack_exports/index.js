function main() {
    return new Promise((resolve) => {
        resolve({
            a: new Promise((resolve2) => {
                resolve2({
                    x: new Promise((resolve3) => {
                        resolve3(99);
                    }),
                    y: "z",
                });
            }),
            b: 42,
            c: {
                d: "a",
                e: false,
            },
        });
    });
}

module.exports = main();
