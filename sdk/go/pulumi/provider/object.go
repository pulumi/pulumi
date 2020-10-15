package provider

import (
	"fmt"
	"reflect"
)

type object interface {
	iterate() objectIterator
	index(key string) (reflect.Value, fieldDesc, bool)
}

type objectIterator interface {
	next() bool
	value() (reflect.Value, fieldDesc)
}

func makeObject(v reflect.Value) (object, error) {
	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map keys must be strings")
		}
		return mapObject{v}, nil
	case reflect.Struct:
		properties := map[string]propertyInfo{}
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if desc, err := getFieldDesc(f); err == nil && desc != nil {
				properties[desc.name] = propertyInfo{index: i, desc: *desc}
			}
		}
		return structObject{v: v, properties: properties}, nil
	default:
		return nil, fmt.Errorf("internal error: cannot treat value of type %v as an object", v.Type())
	}
}

type mapObject struct {
	v reflect.Value
}

func (o mapObject) iterate() objectIterator {
	return &mapObjectIterator{iter: o.v.MapRange()}
}

func (o mapObject) index(key string) (reflect.Value, fieldDesc, bool) {
	keyValue := reflect.New(o.v.Type().Key()).Elem()
	keyValue.SetString(key)

	v := o.v.MapIndex(keyValue)
	if !v.IsValid() {
		return reflect.Value{}, fieldDesc{}, false
	}
	return v, fieldDesc{name: key}, true
}

type mapObjectIterator struct {
	iter *reflect.MapIter
}

func (o *mapObjectIterator) next() bool {
	return o.iter.Next()
}

func (o *mapObjectIterator) value() (reflect.Value, fieldDesc) {
	return o.iter.Value(), fieldDesc{name: o.iter.Key().String()}
}

type propertyInfo struct {
	index int
	desc  fieldDesc
}

type structObject struct {
	v          reflect.Value
	properties map[string]propertyInfo
}

func (o structObject) iterate() objectIterator {
	return &structObjectIterator{v: o.v}
}

func (o structObject) index(key string) (reflect.Value, fieldDesc, bool) {
	property, ok := o.properties[key]
	if !ok {
		return reflect.Value{}, fieldDesc{}, false
	}
	return o.v.Field(property.index), property.desc, true
}

type structObjectIterator struct {
	v reflect.Value
	i int

	field reflect.Value
	desc  fieldDesc
}

func (o *structObjectIterator) next() bool {
	for ; o.i < o.v.NumField(); o.i++ {
		if desc, err := getFieldDesc(o.v.Type().Field(o.i)); err == nil && desc != nil {
			o.i, o.field, o.desc = o.i+1, o.v.Field(o.i), *desc
			return true
		}
	}
	return false
}

func (o *structObjectIterator) value() (reflect.Value, fieldDesc) {
	return o.field, o.desc
}
