import * as pulumi from "pulumi";
import * as crud from "pulumi/crud";

export class Operator implements crud.Provider {
    private op: (l: number, r: number) => any;

    constructor(op: (l: number, r: number) => any) {
        this.op = op;
    }

    check(inputs: any): crud.CheckResult { return new crud.CheckResult(undefined, []); }
    diff(id: pulumi.ID, olds: any, news: any): crud.DiffResult { return new crud.DiffResult([]); }
    delete(id: pulumi.ID, props: any): void { }

    create(inputs: any): crud.CreateResult {
        return new crud.CreateResult("0", this.op(Number(inputs.left), Number(inputs.right)));
    }

    update(id: string, olds: any, news: any): any {
        return new crud.UpdateResult(this.op(Number(news.left), Number(news.right)));
    }
}

export class Div extends Operator {
    constructor() {
        super((left: number, right: number) => <any>{ quotient: Math.floor(left / right), remainder: left % right });
    }

    check(ins: any) {
        return new crud.CheckResult(undefined, ins.right == 0 ? [ new crud.CheckFailure("right", "divisor must be non-zero") ] : []);
    }
}

export var add = new Operator((left: number, right: number) => <any>{ sum: left + right });
export var mul = new Operator((left: number, right: number) => <any>{ product: left * right });
export var sub = new Operator((left: number, right: number) => <any>{ difference: left - right });
export var div = new Div();
