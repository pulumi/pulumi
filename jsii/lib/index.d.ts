export declare class HelloJsii {
    strings: string[];
    constructor();
    baz(input: IOutput<number>): IOutput<string>;
}
export interface IOutput<T> {
    val(): T;
}
export declare type Input<T> = T | Promise<T> | IOutput<T>;
