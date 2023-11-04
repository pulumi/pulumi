import { PropertyDecoder, PropertyValue, ObjectDecoder } from "./propertyValue"
import { z } from "zod"

export const decodeString: PropertyDecoder<string> = {
    decode: (property: PropertyValue) => {
        if (property.kind == "string") {
            return { result: property.value, errors: [] }
        }

        const errorMessage = `Expected a string but instead received a '${property.kind}'`
        return { result: "", errors: [errorMessage] }
    }
}

export function decodeStringOrDefault(defaultValue: string) : PropertyDecoder<string> {
    return {
        decode: (property: PropertyValue) => {
            if (property.kind == "string") {
                return { result: property.value, errors: [] }
            }
            
            if (property.kind == "null" || property.kind == "computed") {
                return { result: defaultValue, errors: [] }
            }

            const errorMessage = `Expected a string but instead received a '${property.kind}'`
            return { result: defaultValue, errors: [errorMessage] }
        }
    }
}

export const decodeNumber: PropertyDecoder<number> = {
    decode: (property: PropertyValue) => {
        if (property.kind == "number") {
            return { result: property.value, errors: [] }
        }

        const errorMessage = `Expected an integer but instead received a '${property.kind}'`
        return { result: 0, errors: [errorMessage] }
    }
}

export function decodeNumberOrDefault(defaultValue: number) : PropertyDecoder<number> {
    return {
        decode: (property: PropertyValue) => {
            if (property.kind == "number") {
                return { result: property.value, errors: [] }
            }
            
            if (property.kind == "null" || property.kind == "computed") {
                return { result: defaultValue, errors: [] }
            }

            const errorMessage = `Expected an integer but instead received a '${property.kind}'`
            return { result: defaultValue, errors: [errorMessage] }
        }
    }
}

export const decodeBool: PropertyDecoder<boolean> = {
    decode: (property: PropertyValue) => {
        if (property.kind == "boolean") {
            return { result: property.value, errors: [] }
        }

        const errorMessage = `Expected a boolean but instead received a '${property.kind}'`
        return { result: false, errors: [errorMessage] }
    }
}

export function decodeBoolOrDefault(defaultValue: boolean) : PropertyDecoder<boolean> {
    return {
        decode: (property: PropertyValue) => {
            if (property.kind == "boolean") {
                return { result: property.value, errors: [] }
            }
            
            if (property.kind == "null" || property.kind == "computed") {
                return { result: defaultValue, errors: [] }
            }

            const errorMessage = `Expected a boolean but instead received a '${property.kind}'`
            return { result: defaultValue, errors: [errorMessage] }
        }
    }
}

export function decodeArray<T>(elementDecoder: PropertyDecoder<T>): PropertyDecoder<T[]> {
    return {
        decode: (property: PropertyValue) => {
            const errors: string[] = []
            const decoded: T[] = []
            if (property.kind === "array") {
                property.values.forEach((element) => {
                    const { result, errors: elementErrors } = elementDecoder.decode(element)
                    decoded.push(result)
                    if (elementErrors.length > 0) {
                        const combinedError = elementErrors.join(", ")
                        errors.push(combinedError)
                    }
                })

                return { result: decoded, errors: errors }
            }

            const errorMessage = `Expected an array but instead received a '${property.kind}'`
            return { result: [], errors: [errorMessage] }
        }
    }
}

export function decodeArrayOrDefault<T>(elementDecoder: PropertyDecoder<T>, defaultArray: T[]): PropertyDecoder<T[]> {
    return {
        decode: (property: PropertyValue) => {
            const errors: string[] = []
            const decoded: T[] = []
            if (property.kind === "array") {
                property.values.forEach((element) => {
                    const { result, errors: elementErrors } = elementDecoder.decode(element)
                    decoded.push(result)
                    if (elementErrors.length > 0) {
                        const combinedError = elementErrors.join(", ")
                        errors.push(combinedError)
                    }
                })

                return { result: decoded, errors: errors }
            }

            if (property.kind === "null" || property.kind === "computed") {
                return { result: defaultArray, errors: [] }
            }

            const errorMessage = `Expected an array but instead received a '${property.kind}'`
            return { result: [], errors: [errorMessage] }
        }
    }
}

export function decodeMap<T>(elementDecoder: PropertyDecoder<T>): PropertyDecoder<Record<string, T>> {
    return {
        decode: (property: PropertyValue) => {
            const errors: string[] = []
            const decoded: Record<string, T> = {}
            if (property.kind === "object") {
                Object.entries(property.values).forEach(([key, value]) => {
                    const { result, errors: elementErrors } = elementDecoder.decode(value)
                    decoded[key] = result

                    if (elementErrors.length > 0) {
                        const combinedError = elementErrors.join(", ")
                        errors.push(combinedError)
                    }
                })

                return { result: decoded, errors: errors }
            }

            const errorMessage = `Expected an object but instead received a '${property.kind}'`
            return { result: {}, errors: [errorMessage] }
        }
    }
}

export function decodeObject<T>(decode: (decoder: ObjectDecoder) => T): PropertyDecoder<T> {
    return {
        decode: (property: PropertyValue) => {
            if (property.kind === "object") {
                try {
                    const decoder = new ObjectDecoder(property.values)
                    return { result: decode(decoder), errors: [] }
                } catch (e) {
                    const error = e as Error
                    return { result: {} as T, errors: [error.message] }
                }
            }
            const errorMessage = `Expected an object but instead received a '${property.kind}'`
            return { result: {} as T, errors: [errorMessage] }
        }
    }
}

export function transformDecoder<T, U>(decoder: PropertyDecoder<T>, transform: (value: T) => U): PropertyDecoder<U> {
    return {
        decode: (property: PropertyValue) => {
            const { result, errors } = decoder.decode(property)
            if (errors.length > 0) {
                return { result: null as any, errors: errors }
            }
            return { result: transform(result), errors: [] }
        }
    }
}


function decodeAnyValue(property: PropertyValue) : any {
    if (property.kind === "string") {
        return { result: property.value, errors: [] }
    }

    if (property.kind === "number") {
        return { result: property.value, errors: [] }
    }

    if (property.kind === "boolean") {
        return { result: property.value, errors: [] }
    }

    if (property.kind === "array") {
        const decoded: any[] = []
        property.values.forEach((element) => {
            const { result, errors } = decodeAnyValue(element)
            decoded.push(result)
        })

        return { result: decoded, errors: [] }
    }

    if (property.kind === "object") {
        const decoded: Record<string, any> = {}
        Object.entries(property.values).forEach(([key, value]) => {
            const { result, errors } = decodeAnyValue(value)
            decoded[key] = result
        })

        return { result: decoded, errors: [] }
    }

    if (property.kind === "asset") {
        return { result: property.value, errors: [] }
    }

    if (property.kind === "archive") {
        return { result: property.value, errors: [] }
    }

    if (property.kind === "secret") {
        return decodeAnyValue(property.value)
    }

    if (property.kind === "output") {
        // TODO: Handle outputs
    }

    return { result: null as any, errors: [] }
}

export const decodeAny: PropertyDecoder<any> = {
    decode: (property: PropertyValue) => {
       return decodeAnyValue(property)
    }
}