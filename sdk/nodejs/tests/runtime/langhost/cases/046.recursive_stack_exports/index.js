function main() {
    var obj = { a: { b: 1 } };
    var obj2 = { x: { y: 1 } };
    obj2.obj2 = obj2;
    obj2.x.x = obj2.x;
    obj2.x.y.x = obj2.x;

    return {
        m: obj,
        n: obj,
        o: obj.a,
        p: obj.a.b,
        obj2: obj2,
        obj2_x: obj2.x,
        obj2_x_y: obj2.x.y,
    };
}

module.exports = main();
