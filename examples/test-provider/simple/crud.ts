export class CRUD {
    check(ins: any): any { return { defaults: undefined, failures: undefined }; }
    diff(id: string, olds: any, news: any): any { return { replaces: undefined }; }
    delete(id: string, props: any): void { }
}

export class Operator extends CRUD {
    private op: (l: number, r: number) => any;

    constructor(op: (l: number, r: number) => any) {
        super();
        this.op = op;
    }

    create(inputs: any): any {
        const result: any = this.op(Number(inputs.left), Number(inputs.right));
        return { id: "0", resource: result, outs: Object.keys(result) };
    }

    update(id: string, olds: any, news: any): any {
        const result: any = this.op(Number(news.left), Number(news.right));
        return { id: id, resource: result, outs: Object.keys(result) };
    }
}

export class Div extends Operator {
    constructor() {
        super(function (left: number, right: number): any { return { quotient: Math.floor(left / right), remainder: left % right } });
    }

    check(ins: any) {
        return { defaults: undefined, failures: ins.right == 0 ? [ { property: "right", reason: "divisor must be non-zero" } ] : undefined };
    }
}

export var add = new Operator(function (left: number, right: number): any { return { sum: left + right }; });
export var mul = new Operator(function (left: number, right: number): any { return { product: left * right }; });
export var sub = new Operator(function (left: number, right: number): any { return { difference: left - right }; });
export var div = new Div();
