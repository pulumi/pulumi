export class HelloJsii {
    // public stringOutput: IOutput<string>;
    // private input: any;

    public strings: string[];

    constructor() {
    }

    baz(input: IOutput<number>): IOutput<string> {
        return <any>input;
    }
}

export interface IOutput<T> {
    val(): T;
}

export type Input<T> = T | Promise<T> | IOutput<T>;

// declare module "." {
//     interface HelloJsii {
//         onObjectCreated(name: string): string;
//     }
// }

// HelloJsii.prototype.onObjectCreated = (name: string) => name;