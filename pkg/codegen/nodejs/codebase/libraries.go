package codebase

var JSMap = BuiltinSymbol{Name: "globalThis.Map"}

func MapT(k Type, v Type) Type {
	return JSMap.AsType().Apply(k, v)
}

var JSPromise = BuiltinSymbol{Name: "globalThis.Promise"}

func PromiseT(t Type) Type {
	return JSPromise.AsType().Apply(t)
}

var JSRecord = BuiltinSymbol{Name: "globalThis.Record"}

func RecordT(k Type, v Type) Type {
	return JSRecord.AsType().Apply(k, v)
}

var JSSet = BuiltinSymbol{Name: "globalThis.Set"}

func SetT(t Type) Type {
	return JSSet.AsType().Apply(t)
}
