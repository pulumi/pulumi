import * as pulumi from "@pulumi/pulumi";

export = async () => {
    const config = new pulumi.Config();
    const aMap = config.requireObject<Record<string, number>>("aMap");
    const anObject = config.requireObject<{prop?: Array<boolean>}>("anObject");
    const anyObject = config.requireObject<any>("anyObject");
    const optionalUntypedObject = config.getObject<any>("optionalUntypedObject") || {
        key: "value",
    };
    const optionalList = config.getObject<Array<string>>("optionalList");
    const optionalMap = config.getObject<Record<string, string>>("optionalMap");
    const optionalObject = config.getObject<{other?: number, prop?: string}>("optionalObject");
    return {
        theMap: {
            a: aMap.a + 1,
            b: aMap.b + 1,
        },
        theObject: anObject.prop?.[0],
        theThing: Number(anyObject.a) + Number(anyObject.b),
        defaultUntypedObject: optionalUntypedObject,
        optionalList: optionalList == null ? "null" : JSON.stringify(optionalList),
        optionalMap: optionalMap == null ? "null" : JSON.stringify(optionalMap),
        optionalObject: optionalObject == null ? "null" : JSON.stringify(optionalObject),
    };
}
