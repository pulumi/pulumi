module.exports = () => {
    return {
        a: Promise.resolve({
            x: Promise.resolve(99),
            y: "z",
        }),
        b: 42,
        c: {
            d: "a",
            e: false,
        },
    };
};
