import { ComponentNamingTransform } from "../..";

export const defaultNamingConvention: ComponentNamingTransform = (args) => {
    return `${args.parent.name}-${args.child.name}`;
};
