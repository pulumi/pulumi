import { Asset, Archive } from "../asset"

export type PropertyValue =
  | { "kind": "string", value: string }
  | { "kind": "number", value: number }
  | { "kind": "boolean", value: boolean }
  | { "kind": "object", values: Record<string, PropertyValue> }
  | { "kind": "array", values: PropertyValue[] }
  | { "kind": "asset", value: Asset }
  | { "kind": "archive", value: Archive }
  | { "kind": "secret", value: PropertyValue }
  | { "kind": "output", value: PropertyValue, dependencies: string[] }
  | { "kind": "computed" }
  | { "kind": "null" }

export type DecodeResult<T> = {
    result: T,
    errors: string[]
}

export interface PropertyDecoder<T> {
    decode: (value: PropertyValue) => DecodeResult<T>
}

export class ObjectDecoder {
    private readonly data: Record<string, PropertyValue>
    constructor(data: Record<string, PropertyValue>) {
        this.data = data
    }

    public field<T>(name: string, decoder: PropertyDecoder<T>): T {
        const property = this.data[name]
        const { result, errors } = decoder.decode(property)
        if (errors.length > 0) {
            throw new Error(`Error decoding field '${name}': ${errors.join(", ")}`)
        }

        return result
    }

    public optionalField<T>(name: string, decoder: PropertyDecoder<T>): T | undefined {
        const property = this.data[name]
        if (property === undefined) {
            return undefined
        }

        if (property.kind === "null" || property.kind === "computed")  {
            return undefined
        }

        const { result, errors } = decoder.decode(property)
        if (errors.length > 0) {
            throw new Error(`Error decoding field '${name}': ${errors.join(", ")}`)
        }

        return result
    }
}