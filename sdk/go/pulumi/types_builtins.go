// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// nolint: lll, interfacer
package pulumi

import (
	"reflect"
)

// ApplyArchive is like ApplyT, but returns a ArchiveOutput.
func (o *OutputState) ApplyArchive(applier interface{}) ArchiveOutput {
	return o.ApplyT(applier).(ArchiveOutput)
}

// ApplyArchiveArray is like ApplyT, but returns a ArchiveArrayOutput.
func (o *OutputState) ApplyArchiveArray(applier interface{}) ArchiveArrayOutput {
	return o.ApplyT(applier).(ArchiveArrayOutput)
}

// ApplyArchiveMap is like ApplyT, but returns a ArchiveMapOutput.
func (o *OutputState) ApplyArchiveMap(applier interface{}) ArchiveMapOutput {
	return o.ApplyT(applier).(ArchiveMapOutput)
}

// ApplyArchiveArrayMap is like ApplyT, but returns a ArchiveArrayMapOutput.
func (o *OutputState) ApplyArchiveArrayMap(applier interface{}) ArchiveArrayMapOutput {
	return o.ApplyT(applier).(ArchiveArrayMapOutput)
}

// ApplyArchiveMapArray is like ApplyT, but returns a ArchiveMapArrayOutput.
func (o *OutputState) ApplyArchiveMapArray(applier interface{}) ArchiveMapArrayOutput {
	return o.ApplyT(applier).(ArchiveMapArrayOutput)
}

// ApplyAsset is like ApplyT, but returns a AssetOutput.
func (o *OutputState) ApplyAsset(applier interface{}) AssetOutput {
	return o.ApplyT(applier).(AssetOutput)
}

// ApplyAssetArray is like ApplyT, but returns a AssetArrayOutput.
func (o *OutputState) ApplyAssetArray(applier interface{}) AssetArrayOutput {
	return o.ApplyT(applier).(AssetArrayOutput)
}

// ApplyAssetMap is like ApplyT, but returns a AssetMapOutput.
func (o *OutputState) ApplyAssetMap(applier interface{}) AssetMapOutput {
	return o.ApplyT(applier).(AssetMapOutput)
}

// ApplyAssetArrayMap is like ApplyT, but returns a AssetArrayMapOutput.
func (o *OutputState) ApplyAssetArrayMap(applier interface{}) AssetArrayMapOutput {
	return o.ApplyT(applier).(AssetArrayMapOutput)
}

// ApplyAssetMapArray is like ApplyT, but returns a AssetMapArrayOutput.
func (o *OutputState) ApplyAssetMapArray(applier interface{}) AssetMapArrayOutput {
	return o.ApplyT(applier).(AssetMapArrayOutput)
}

// ApplyAssetOrArchive is like ApplyT, but returns a AssetOrArchiveOutput.
func (o *OutputState) ApplyAssetOrArchive(applier interface{}) AssetOrArchiveOutput {
	return o.ApplyT(applier).(AssetOrArchiveOutput)
}

// ApplyAssetOrArchiveArray is like ApplyT, but returns a AssetOrArchiveArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveArray(applier interface{}) AssetOrArchiveArrayOutput {
	return o.ApplyT(applier).(AssetOrArchiveArrayOutput)
}

// ApplyAssetOrArchiveMap is like ApplyT, but returns a AssetOrArchiveMapOutput.
func (o *OutputState) ApplyAssetOrArchiveMap(applier interface{}) AssetOrArchiveMapOutput {
	return o.ApplyT(applier).(AssetOrArchiveMapOutput)
}

// ApplyAssetOrArchiveArrayMap is like ApplyT, but returns a AssetOrArchiveArrayMapOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayMap(applier interface{}) AssetOrArchiveArrayMapOutput {
	return o.ApplyT(applier).(AssetOrArchiveArrayMapOutput)
}

// ApplyAssetOrArchiveMapArray is like ApplyT, but returns a AssetOrArchiveMapArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveMapArray(applier interface{}) AssetOrArchiveMapArrayOutput {
	return o.ApplyT(applier).(AssetOrArchiveMapArrayOutput)
}

// ApplyBool is like ApplyT, but returns a BoolOutput.
func (o *OutputState) ApplyBool(applier interface{}) BoolOutput {
	return o.ApplyT(applier).(BoolOutput)
}

// ApplyBoolPtr is like ApplyT, but returns a BoolPtrOutput.
func (o *OutputState) ApplyBoolPtr(applier interface{}) BoolPtrOutput {
	return o.ApplyT(applier).(BoolPtrOutput)
}

// ApplyBoolArray is like ApplyT, but returns a BoolArrayOutput.
func (o *OutputState) ApplyBoolArray(applier interface{}) BoolArrayOutput {
	return o.ApplyT(applier).(BoolArrayOutput)
}

// ApplyBoolMap is like ApplyT, but returns a BoolMapOutput.
func (o *OutputState) ApplyBoolMap(applier interface{}) BoolMapOutput {
	return o.ApplyT(applier).(BoolMapOutput)
}

// ApplyBoolArrayMap is like ApplyT, but returns a BoolArrayMapOutput.
func (o *OutputState) ApplyBoolArrayMap(applier interface{}) BoolArrayMapOutput {
	return o.ApplyT(applier).(BoolArrayMapOutput)
}

// ApplyBoolMapArray is like ApplyT, but returns a BoolMapArrayOutput.
func (o *OutputState) ApplyBoolMapArray(applier interface{}) BoolMapArrayOutput {
	return o.ApplyT(applier).(BoolMapArrayOutput)
}

// ApplyFloat32 is like ApplyT, but returns a Float32Output.
func (o *OutputState) ApplyFloat32(applier interface{}) Float32Output {
	return o.ApplyT(applier).(Float32Output)
}

// ApplyFloat32Ptr is like ApplyT, but returns a Float32PtrOutput.
func (o *OutputState) ApplyFloat32Ptr(applier interface{}) Float32PtrOutput {
	return o.ApplyT(applier).(Float32PtrOutput)
}

// ApplyFloat32Array is like ApplyT, but returns a Float32ArrayOutput.
func (o *OutputState) ApplyFloat32Array(applier interface{}) Float32ArrayOutput {
	return o.ApplyT(applier).(Float32ArrayOutput)
}

// ApplyFloat32Map is like ApplyT, but returns a Float32MapOutput.
func (o *OutputState) ApplyFloat32Map(applier interface{}) Float32MapOutput {
	return o.ApplyT(applier).(Float32MapOutput)
}

// ApplyFloat32ArrayMap is like ApplyT, but returns a Float32ArrayMapOutput.
func (o *OutputState) ApplyFloat32ArrayMap(applier interface{}) Float32ArrayMapOutput {
	return o.ApplyT(applier).(Float32ArrayMapOutput)
}

// ApplyFloat32MapArray is like ApplyT, but returns a Float32MapArrayOutput.
func (o *OutputState) ApplyFloat32MapArray(applier interface{}) Float32MapArrayOutput {
	return o.ApplyT(applier).(Float32MapArrayOutput)
}

// ApplyFloat64 is like ApplyT, but returns a Float64Output.
func (o *OutputState) ApplyFloat64(applier interface{}) Float64Output {
	return o.ApplyT(applier).(Float64Output)
}

// ApplyFloat64Ptr is like ApplyT, but returns a Float64PtrOutput.
func (o *OutputState) ApplyFloat64Ptr(applier interface{}) Float64PtrOutput {
	return o.ApplyT(applier).(Float64PtrOutput)
}

// ApplyFloat64Array is like ApplyT, but returns a Float64ArrayOutput.
func (o *OutputState) ApplyFloat64Array(applier interface{}) Float64ArrayOutput {
	return o.ApplyT(applier).(Float64ArrayOutput)
}

// ApplyFloat64Map is like ApplyT, but returns a Float64MapOutput.
func (o *OutputState) ApplyFloat64Map(applier interface{}) Float64MapOutput {
	return o.ApplyT(applier).(Float64MapOutput)
}

// ApplyFloat64ArrayMap is like ApplyT, but returns a Float64ArrayMapOutput.
func (o *OutputState) ApplyFloat64ArrayMap(applier interface{}) Float64ArrayMapOutput {
	return o.ApplyT(applier).(Float64ArrayMapOutput)
}

// ApplyFloat64MapArray is like ApplyT, but returns a Float64MapArrayOutput.
func (o *OutputState) ApplyFloat64MapArray(applier interface{}) Float64MapArrayOutput {
	return o.ApplyT(applier).(Float64MapArrayOutput)
}

// ApplyID is like ApplyT, but returns a IDOutput.
func (o *OutputState) ApplyID(applier interface{}) IDOutput {
	return o.ApplyT(applier).(IDOutput)
}

// ApplyIDPtr is like ApplyT, but returns a IDPtrOutput.
func (o *OutputState) ApplyIDPtr(applier interface{}) IDPtrOutput {
	return o.ApplyT(applier).(IDPtrOutput)
}

// ApplyIDArray is like ApplyT, but returns a IDArrayOutput.
func (o *OutputState) ApplyIDArray(applier interface{}) IDArrayOutput {
	return o.ApplyT(applier).(IDArrayOutput)
}

// ApplyIDMap is like ApplyT, but returns a IDMapOutput.
func (o *OutputState) ApplyIDMap(applier interface{}) IDMapOutput {
	return o.ApplyT(applier).(IDMapOutput)
}

// ApplyIDArrayMap is like ApplyT, but returns a IDArrayMapOutput.
func (o *OutputState) ApplyIDArrayMap(applier interface{}) IDArrayMapOutput {
	return o.ApplyT(applier).(IDArrayMapOutput)
}

// ApplyIDMapArray is like ApplyT, but returns a IDMapArrayOutput.
func (o *OutputState) ApplyIDMapArray(applier interface{}) IDMapArrayOutput {
	return o.ApplyT(applier).(IDMapArrayOutput)
}

// ApplyArray is like ApplyT, but returns a ArrayOutput.
func (o *OutputState) ApplyArray(applier interface{}) ArrayOutput {
	return o.ApplyT(applier).(ArrayOutput)
}

// ApplyMap is like ApplyT, but returns a MapOutput.
func (o *OutputState) ApplyMap(applier interface{}) MapOutput {
	return o.ApplyT(applier).(MapOutput)
}

// ApplyArrayMap is like ApplyT, but returns a ArrayMapOutput.
func (o *OutputState) ApplyArrayMap(applier interface{}) ArrayMapOutput {
	return o.ApplyT(applier).(ArrayMapOutput)
}

// ApplyMapArray is like ApplyT, but returns a MapArrayOutput.
func (o *OutputState) ApplyMapArray(applier interface{}) MapArrayOutput {
	return o.ApplyT(applier).(MapArrayOutput)
}

// ApplyInt is like ApplyT, but returns a IntOutput.
func (o *OutputState) ApplyInt(applier interface{}) IntOutput {
	return o.ApplyT(applier).(IntOutput)
}

// ApplyIntPtr is like ApplyT, but returns a IntPtrOutput.
func (o *OutputState) ApplyIntPtr(applier interface{}) IntPtrOutput {
	return o.ApplyT(applier).(IntPtrOutput)
}

// ApplyIntArray is like ApplyT, but returns a IntArrayOutput.
func (o *OutputState) ApplyIntArray(applier interface{}) IntArrayOutput {
	return o.ApplyT(applier).(IntArrayOutput)
}

// ApplyIntMap is like ApplyT, but returns a IntMapOutput.
func (o *OutputState) ApplyIntMap(applier interface{}) IntMapOutput {
	return o.ApplyT(applier).(IntMapOutput)
}

// ApplyIntArrayMap is like ApplyT, but returns a IntArrayMapOutput.
func (o *OutputState) ApplyIntArrayMap(applier interface{}) IntArrayMapOutput {
	return o.ApplyT(applier).(IntArrayMapOutput)
}

// ApplyIntMapArray is like ApplyT, but returns a IntMapArrayOutput.
func (o *OutputState) ApplyIntMapArray(applier interface{}) IntMapArrayOutput {
	return o.ApplyT(applier).(IntMapArrayOutput)
}

// ApplyInt16 is like ApplyT, but returns a Int16Output.
func (o *OutputState) ApplyInt16(applier interface{}) Int16Output {
	return o.ApplyT(applier).(Int16Output)
}

// ApplyInt16Ptr is like ApplyT, but returns a Int16PtrOutput.
func (o *OutputState) ApplyInt16Ptr(applier interface{}) Int16PtrOutput {
	return o.ApplyT(applier).(Int16PtrOutput)
}

// ApplyInt16Array is like ApplyT, but returns a Int16ArrayOutput.
func (o *OutputState) ApplyInt16Array(applier interface{}) Int16ArrayOutput {
	return o.ApplyT(applier).(Int16ArrayOutput)
}

// ApplyInt16Map is like ApplyT, but returns a Int16MapOutput.
func (o *OutputState) ApplyInt16Map(applier interface{}) Int16MapOutput {
	return o.ApplyT(applier).(Int16MapOutput)
}

// ApplyInt16ArrayMap is like ApplyT, but returns a Int16ArrayMapOutput.
func (o *OutputState) ApplyInt16ArrayMap(applier interface{}) Int16ArrayMapOutput {
	return o.ApplyT(applier).(Int16ArrayMapOutput)
}

// ApplyInt16MapArray is like ApplyT, but returns a Int16MapArrayOutput.
func (o *OutputState) ApplyInt16MapArray(applier interface{}) Int16MapArrayOutput {
	return o.ApplyT(applier).(Int16MapArrayOutput)
}

// ApplyInt32 is like ApplyT, but returns a Int32Output.
func (o *OutputState) ApplyInt32(applier interface{}) Int32Output {
	return o.ApplyT(applier).(Int32Output)
}

// ApplyInt32Ptr is like ApplyT, but returns a Int32PtrOutput.
func (o *OutputState) ApplyInt32Ptr(applier interface{}) Int32PtrOutput {
	return o.ApplyT(applier).(Int32PtrOutput)
}

// ApplyInt32Array is like ApplyT, but returns a Int32ArrayOutput.
func (o *OutputState) ApplyInt32Array(applier interface{}) Int32ArrayOutput {
	return o.ApplyT(applier).(Int32ArrayOutput)
}

// ApplyInt32Map is like ApplyT, but returns a Int32MapOutput.
func (o *OutputState) ApplyInt32Map(applier interface{}) Int32MapOutput {
	return o.ApplyT(applier).(Int32MapOutput)
}

// ApplyInt32ArrayMap is like ApplyT, but returns a Int32ArrayMapOutput.
func (o *OutputState) ApplyInt32ArrayMap(applier interface{}) Int32ArrayMapOutput {
	return o.ApplyT(applier).(Int32ArrayMapOutput)
}

// ApplyInt32MapArray is like ApplyT, but returns a Int32MapArrayOutput.
func (o *OutputState) ApplyInt32MapArray(applier interface{}) Int32MapArrayOutput {
	return o.ApplyT(applier).(Int32MapArrayOutput)
}

// ApplyInt64 is like ApplyT, but returns a Int64Output.
func (o *OutputState) ApplyInt64(applier interface{}) Int64Output {
	return o.ApplyT(applier).(Int64Output)
}

// ApplyInt64Ptr is like ApplyT, but returns a Int64PtrOutput.
func (o *OutputState) ApplyInt64Ptr(applier interface{}) Int64PtrOutput {
	return o.ApplyT(applier).(Int64PtrOutput)
}

// ApplyInt64Array is like ApplyT, but returns a Int64ArrayOutput.
func (o *OutputState) ApplyInt64Array(applier interface{}) Int64ArrayOutput {
	return o.ApplyT(applier).(Int64ArrayOutput)
}

// ApplyInt64Map is like ApplyT, but returns a Int64MapOutput.
func (o *OutputState) ApplyInt64Map(applier interface{}) Int64MapOutput {
	return o.ApplyT(applier).(Int64MapOutput)
}

// ApplyInt64ArrayMap is like ApplyT, but returns a Int64ArrayMapOutput.
func (o *OutputState) ApplyInt64ArrayMap(applier interface{}) Int64ArrayMapOutput {
	return o.ApplyT(applier).(Int64ArrayMapOutput)
}

// ApplyInt64MapArray is like ApplyT, but returns a Int64MapArrayOutput.
func (o *OutputState) ApplyInt64MapArray(applier interface{}) Int64MapArrayOutput {
	return o.ApplyT(applier).(Int64MapArrayOutput)
}

// ApplyInt8 is like ApplyT, but returns a Int8Output.
func (o *OutputState) ApplyInt8(applier interface{}) Int8Output {
	return o.ApplyT(applier).(Int8Output)
}

// ApplyInt8Ptr is like ApplyT, but returns a Int8PtrOutput.
func (o *OutputState) ApplyInt8Ptr(applier interface{}) Int8PtrOutput {
	return o.ApplyT(applier).(Int8PtrOutput)
}

// ApplyInt8Array is like ApplyT, but returns a Int8ArrayOutput.
func (o *OutputState) ApplyInt8Array(applier interface{}) Int8ArrayOutput {
	return o.ApplyT(applier).(Int8ArrayOutput)
}

// ApplyInt8Map is like ApplyT, but returns a Int8MapOutput.
func (o *OutputState) ApplyInt8Map(applier interface{}) Int8MapOutput {
	return o.ApplyT(applier).(Int8MapOutput)
}

// ApplyInt8ArrayMap is like ApplyT, but returns a Int8ArrayMapOutput.
func (o *OutputState) ApplyInt8ArrayMap(applier interface{}) Int8ArrayMapOutput {
	return o.ApplyT(applier).(Int8ArrayMapOutput)
}

// ApplyInt8MapArray is like ApplyT, but returns a Int8MapArrayOutput.
func (o *OutputState) ApplyInt8MapArray(applier interface{}) Int8MapArrayOutput {
	return o.ApplyT(applier).(Int8MapArrayOutput)
}

// ApplyString is like ApplyT, but returns a StringOutput.
func (o *OutputState) ApplyString(applier interface{}) StringOutput {
	return o.ApplyT(applier).(StringOutput)
}

// ApplyStringPtr is like ApplyT, but returns a StringPtrOutput.
func (o *OutputState) ApplyStringPtr(applier interface{}) StringPtrOutput {
	return o.ApplyT(applier).(StringPtrOutput)
}

// ApplyStringArray is like ApplyT, but returns a StringArrayOutput.
func (o *OutputState) ApplyStringArray(applier interface{}) StringArrayOutput {
	return o.ApplyT(applier).(StringArrayOutput)
}

// ApplyStringMap is like ApplyT, but returns a StringMapOutput.
func (o *OutputState) ApplyStringMap(applier interface{}) StringMapOutput {
	return o.ApplyT(applier).(StringMapOutput)
}

// ApplyStringArrayMap is like ApplyT, but returns a StringArrayMapOutput.
func (o *OutputState) ApplyStringArrayMap(applier interface{}) StringArrayMapOutput {
	return o.ApplyT(applier).(StringArrayMapOutput)
}

// ApplyStringMapArray is like ApplyT, but returns a StringMapArrayOutput.
func (o *OutputState) ApplyStringMapArray(applier interface{}) StringMapArrayOutput {
	return o.ApplyT(applier).(StringMapArrayOutput)
}

// ApplyURN is like ApplyT, but returns a URNOutput.
func (o *OutputState) ApplyURN(applier interface{}) URNOutput {
	return o.ApplyT(applier).(URNOutput)
}

// ApplyURNPtr is like ApplyT, but returns a URNPtrOutput.
func (o *OutputState) ApplyURNPtr(applier interface{}) URNPtrOutput {
	return o.ApplyT(applier).(URNPtrOutput)
}

// ApplyURNArray is like ApplyT, but returns a URNArrayOutput.
func (o *OutputState) ApplyURNArray(applier interface{}) URNArrayOutput {
	return o.ApplyT(applier).(URNArrayOutput)
}

// ApplyURNMap is like ApplyT, but returns a URNMapOutput.
func (o *OutputState) ApplyURNMap(applier interface{}) URNMapOutput {
	return o.ApplyT(applier).(URNMapOutput)
}

// ApplyURNArrayMap is like ApplyT, but returns a URNArrayMapOutput.
func (o *OutputState) ApplyURNArrayMap(applier interface{}) URNArrayMapOutput {
	return o.ApplyT(applier).(URNArrayMapOutput)
}

// ApplyURNMapArray is like ApplyT, but returns a URNMapArrayOutput.
func (o *OutputState) ApplyURNMapArray(applier interface{}) URNMapArrayOutput {
	return o.ApplyT(applier).(URNMapArrayOutput)
}

// ApplyUint is like ApplyT, but returns a UintOutput.
func (o *OutputState) ApplyUint(applier interface{}) UintOutput {
	return o.ApplyT(applier).(UintOutput)
}

// ApplyUintPtr is like ApplyT, but returns a UintPtrOutput.
func (o *OutputState) ApplyUintPtr(applier interface{}) UintPtrOutput {
	return o.ApplyT(applier).(UintPtrOutput)
}

// ApplyUintArray is like ApplyT, but returns a UintArrayOutput.
func (o *OutputState) ApplyUintArray(applier interface{}) UintArrayOutput {
	return o.ApplyT(applier).(UintArrayOutput)
}

// ApplyUintMap is like ApplyT, but returns a UintMapOutput.
func (o *OutputState) ApplyUintMap(applier interface{}) UintMapOutput {
	return o.ApplyT(applier).(UintMapOutput)
}

// ApplyUintArrayMap is like ApplyT, but returns a UintArrayMapOutput.
func (o *OutputState) ApplyUintArrayMap(applier interface{}) UintArrayMapOutput {
	return o.ApplyT(applier).(UintArrayMapOutput)
}

// ApplyUintMapArray is like ApplyT, but returns a UintMapArrayOutput.
func (o *OutputState) ApplyUintMapArray(applier interface{}) UintMapArrayOutput {
	return o.ApplyT(applier).(UintMapArrayOutput)
}

// ApplyUint16 is like ApplyT, but returns a Uint16Output.
func (o *OutputState) ApplyUint16(applier interface{}) Uint16Output {
	return o.ApplyT(applier).(Uint16Output)
}

// ApplyUint16Ptr is like ApplyT, but returns a Uint16PtrOutput.
func (o *OutputState) ApplyUint16Ptr(applier interface{}) Uint16PtrOutput {
	return o.ApplyT(applier).(Uint16PtrOutput)
}

// ApplyUint16Array is like ApplyT, but returns a Uint16ArrayOutput.
func (o *OutputState) ApplyUint16Array(applier interface{}) Uint16ArrayOutput {
	return o.ApplyT(applier).(Uint16ArrayOutput)
}

// ApplyUint16Map is like ApplyT, but returns a Uint16MapOutput.
func (o *OutputState) ApplyUint16Map(applier interface{}) Uint16MapOutput {
	return o.ApplyT(applier).(Uint16MapOutput)
}

// ApplyUint16ArrayMap is like ApplyT, but returns a Uint16ArrayMapOutput.
func (o *OutputState) ApplyUint16ArrayMap(applier interface{}) Uint16ArrayMapOutput {
	return o.ApplyT(applier).(Uint16ArrayMapOutput)
}

// ApplyUint16MapArray is like ApplyT, but returns a Uint16MapArrayOutput.
func (o *OutputState) ApplyUint16MapArray(applier interface{}) Uint16MapArrayOutput {
	return o.ApplyT(applier).(Uint16MapArrayOutput)
}

// ApplyUint32 is like ApplyT, but returns a Uint32Output.
func (o *OutputState) ApplyUint32(applier interface{}) Uint32Output {
	return o.ApplyT(applier).(Uint32Output)
}

// ApplyUint32Ptr is like ApplyT, but returns a Uint32PtrOutput.
func (o *OutputState) ApplyUint32Ptr(applier interface{}) Uint32PtrOutput {
	return o.ApplyT(applier).(Uint32PtrOutput)
}

// ApplyUint32Array is like ApplyT, but returns a Uint32ArrayOutput.
func (o *OutputState) ApplyUint32Array(applier interface{}) Uint32ArrayOutput {
	return o.ApplyT(applier).(Uint32ArrayOutput)
}

// ApplyUint32Map is like ApplyT, but returns a Uint32MapOutput.
func (o *OutputState) ApplyUint32Map(applier interface{}) Uint32MapOutput {
	return o.ApplyT(applier).(Uint32MapOutput)
}

// ApplyUint32ArrayMap is like ApplyT, but returns a Uint32ArrayMapOutput.
func (o *OutputState) ApplyUint32ArrayMap(applier interface{}) Uint32ArrayMapOutput {
	return o.ApplyT(applier).(Uint32ArrayMapOutput)
}

// ApplyUint32MapArray is like ApplyT, but returns a Uint32MapArrayOutput.
func (o *OutputState) ApplyUint32MapArray(applier interface{}) Uint32MapArrayOutput {
	return o.ApplyT(applier).(Uint32MapArrayOutput)
}

// ApplyUint64 is like ApplyT, but returns a Uint64Output.
func (o *OutputState) ApplyUint64(applier interface{}) Uint64Output {
	return o.ApplyT(applier).(Uint64Output)
}

// ApplyUint64Ptr is like ApplyT, but returns a Uint64PtrOutput.
func (o *OutputState) ApplyUint64Ptr(applier interface{}) Uint64PtrOutput {
	return o.ApplyT(applier).(Uint64PtrOutput)
}

// ApplyUint64Array is like ApplyT, but returns a Uint64ArrayOutput.
func (o *OutputState) ApplyUint64Array(applier interface{}) Uint64ArrayOutput {
	return o.ApplyT(applier).(Uint64ArrayOutput)
}

// ApplyUint64Map is like ApplyT, but returns a Uint64MapOutput.
func (o *OutputState) ApplyUint64Map(applier interface{}) Uint64MapOutput {
	return o.ApplyT(applier).(Uint64MapOutput)
}

// ApplyUint64ArrayMap is like ApplyT, but returns a Uint64ArrayMapOutput.
func (o *OutputState) ApplyUint64ArrayMap(applier interface{}) Uint64ArrayMapOutput {
	return o.ApplyT(applier).(Uint64ArrayMapOutput)
}

// ApplyUint64MapArray is like ApplyT, but returns a Uint64MapArrayOutput.
func (o *OutputState) ApplyUint64MapArray(applier interface{}) Uint64MapArrayOutput {
	return o.ApplyT(applier).(Uint64MapArrayOutput)
}

// ApplyUint8 is like ApplyT, but returns a Uint8Output.
func (o *OutputState) ApplyUint8(applier interface{}) Uint8Output {
	return o.ApplyT(applier).(Uint8Output)
}

// ApplyUint8Ptr is like ApplyT, but returns a Uint8PtrOutput.
func (o *OutputState) ApplyUint8Ptr(applier interface{}) Uint8PtrOutput {
	return o.ApplyT(applier).(Uint8PtrOutput)
}

// ApplyUint8Array is like ApplyT, but returns a Uint8ArrayOutput.
func (o *OutputState) ApplyUint8Array(applier interface{}) Uint8ArrayOutput {
	return o.ApplyT(applier).(Uint8ArrayOutput)
}

// ApplyUint8Map is like ApplyT, but returns a Uint8MapOutput.
func (o *OutputState) ApplyUint8Map(applier interface{}) Uint8MapOutput {
	return o.ApplyT(applier).(Uint8MapOutput)
}

// ApplyUint8ArrayMap is like ApplyT, but returns a Uint8ArrayMapOutput.
func (o *OutputState) ApplyUint8ArrayMap(applier interface{}) Uint8ArrayMapOutput {
	return o.ApplyT(applier).(Uint8ArrayMapOutput)
}

// ApplyUint8MapArray is like ApplyT, but returns a Uint8MapArrayOutput.
func (o *OutputState) ApplyUint8MapArray(applier interface{}) Uint8MapArrayOutput {
	return o.ApplyT(applier).(Uint8MapArrayOutput)
}

var archiveType = reflect.TypeOf((*Archive)(nil)).Elem()

// ArchiveInput is an input type that accepts Archive and ArchiveOutput values.
type ArchiveInput interface {
	Input

	ToArchiveOutput() ArchiveOutput
}

// ElementType returns the element type of this Input (Archive).
func (*archive) ElementType() reflect.Type {
	return archiveType
}

func (in *archive) ToArchiveOutput() ArchiveOutput {
	return ToOutput(in).(ArchiveOutput)
}

func (in *archive) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return in.ToArchiveOutput().ToAssetOrArchiveOutput()
}

// ArchiveOutput is an Output that returns Archive values.
type ArchiveOutput struct{ *OutputState }

// ElementType returns the element type of this Output (Archive).
func (ArchiveOutput) ElementType() reflect.Type {
	return archiveType
}

func (o ArchiveOutput) ToArchiveOutput() ArchiveOutput {
	return o
}

func (o ArchiveOutput) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return o.ApplyT(func(v Archive) AssetOrArchive {
		return (AssetOrArchive)(v)
	}).(AssetOrArchiveOutput)
}

var archiveArrayType = reflect.TypeOf((*[]Archive)(nil)).Elem()

// ArchiveArrayInput is an input type that accepts ArchiveArray and ArchiveArrayOutput values.
type ArchiveArrayInput interface {
	Input

	ToArchiveArrayOutput() ArchiveArrayOutput
}

// ArchiveArray is an input type for []ArchiveInput values.
type ArchiveArray []ArchiveInput

// ElementType returns the element type of this Input ([]Archive).
func (ArchiveArray) ElementType() reflect.Type {
	return archiveArrayType
}

func (in ArchiveArray) ToArchiveArrayOutput() ArchiveArrayOutput {
	return ToOutput(in).(ArchiveArrayOutput)
}

// ArchiveArrayOutput is an Output that returns []Archive values.
type ArchiveArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]Archive).
func (ArchiveArrayOutput) ElementType() reflect.Type {
	return archiveArrayType
}

func (o ArchiveArrayOutput) ToArchiveArrayOutput() ArchiveArrayOutput {
	return o
}

func (o ArchiveArrayOutput) Index(i IntInput) ArchiveOutput {
	return All(o, i).ApplyT(func(vs []interface{}) Archive {
		return vs[0].([]Archive)[vs[1].(int)]
	}).(ArchiveOutput)
}

var archiveMapType = reflect.TypeOf((*map[string]Archive)(nil)).Elem()

// ArchiveMapInput is an input type that accepts ArchiveMap and ArchiveMapOutput values.
type ArchiveMapInput interface {
	Input

	ToArchiveMapOutput() ArchiveMapOutput
}

// ArchiveMap is an input type for map[string]ArchiveInput values.
type ArchiveMap map[string]ArchiveInput

// ElementType returns the element type of this Input (map[string]Archive).
func (ArchiveMap) ElementType() reflect.Type {
	return archiveMapType
}

func (in ArchiveMap) ToArchiveMapOutput() ArchiveMapOutput {
	return ToOutput(in).(ArchiveMapOutput)
}

// ArchiveMapOutput is an Output that returns map[string]Archive values.
type ArchiveMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]Archive).
func (ArchiveMapOutput) ElementType() reflect.Type {
	return archiveMapType
}

func (o ArchiveMapOutput) ToArchiveMapOutput() ArchiveMapOutput {
	return o
}

func (o ArchiveMapOutput) MapIndex(k StringInput) ArchiveOutput {
	return All(o, k).ApplyT(func(vs []interface{}) Archive {
		return vs[0].(map[string]Archive)[vs[1].(string)]
	}).(ArchiveOutput)
}

var archiveArrayMapType = reflect.TypeOf((*map[string][]Archive)(nil)).Elem()

// ArchiveArrayMapInput is an input type that accepts ArchiveArrayMap and ArchiveArrayMapOutput values.
type ArchiveArrayMapInput interface {
	Input

	ToArchiveArrayMapOutput() ArchiveArrayMapOutput
}

// ArchiveArrayMap is an input type for map[string]ArchiveArrayInput values.
type ArchiveArrayMap map[string]ArchiveArrayInput

// ElementType returns the element type of this Input (map[string][]Archive).
func (ArchiveArrayMap) ElementType() reflect.Type {
	return archiveArrayMapType
}

func (in ArchiveArrayMap) ToArchiveArrayMapOutput() ArchiveArrayMapOutput {
	return ToOutput(in).(ArchiveArrayMapOutput)
}

// ArchiveArrayMapOutput is an Output that returns map[string][]Archive values.
type ArchiveArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]Archive).
func (ArchiveArrayMapOutput) ElementType() reflect.Type {
	return archiveArrayMapType
}

func (o ArchiveArrayMapOutput) ToArchiveArrayMapOutput() ArchiveArrayMapOutput {
	return o
}

func (o ArchiveArrayMapOutput) MapIndex(k StringInput) ArchiveArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []Archive {
		return vs[0].(map[string][]Archive)[vs[1].(string)]
	}).(ArchiveArrayOutput)
}

var archiveMapArrayType = reflect.TypeOf((*[]map[string]Archive)(nil)).Elem()

// ArchiveMapArrayInput is an input type that accepts ArchiveMapArray and ArchiveMapArrayOutput values.
type ArchiveMapArrayInput interface {
	Input

	ToArchiveMapArrayOutput() ArchiveMapArrayOutput
}

// ArchiveMapArray is an input type for []ArchiveMapInput values.
type ArchiveMapArray []ArchiveMapInput

// ElementType returns the element type of this Input ([]map[string]Archive).
func (ArchiveMapArray) ElementType() reflect.Type {
	return archiveMapArrayType
}

func (in ArchiveMapArray) ToArchiveMapArrayOutput() ArchiveMapArrayOutput {
	return ToOutput(in).(ArchiveMapArrayOutput)
}

// ArchiveMapArrayOutput is an Output that returns []map[string]Archive values.
type ArchiveMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]Archive).
func (ArchiveMapArrayOutput) ElementType() reflect.Type {
	return archiveMapArrayType
}

func (o ArchiveMapArrayOutput) ToArchiveMapArrayOutput() ArchiveMapArrayOutput {
	return o
}

func (o ArchiveMapArrayOutput) Index(i IntInput) ArchiveMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]Archive {
		return vs[0].([]map[string]Archive)[vs[1].(int)]
	}).(ArchiveMapOutput)
}

var assetType = reflect.TypeOf((*Asset)(nil)).Elem()

// AssetInput is an input type that accepts Asset and AssetOutput values.
type AssetInput interface {
	Input

	ToAssetOutput() AssetOutput
}

// ElementType returns the element type of this Input (Asset).
func (*asset) ElementType() reflect.Type {
	return assetType
}

func (in *asset) ToAssetOutput() AssetOutput {
	return ToOutput(in).(AssetOutput)
}

func (in *asset) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return in.ToAssetOutput().ToAssetOrArchiveOutput()
}

// AssetOutput is an Output that returns Asset values.
type AssetOutput struct{ *OutputState }

// ElementType returns the element type of this Output (Asset).
func (AssetOutput) ElementType() reflect.Type {
	return assetType
}

func (o AssetOutput) ToAssetOutput() AssetOutput {
	return o
}

func (o AssetOutput) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return o.ApplyT(func(v Asset) AssetOrArchive {
		return (AssetOrArchive)(v)
	}).(AssetOrArchiveOutput)
}

var assetArrayType = reflect.TypeOf((*[]Asset)(nil)).Elem()

// AssetArrayInput is an input type that accepts AssetArray and AssetArrayOutput values.
type AssetArrayInput interface {
	Input

	ToAssetArrayOutput() AssetArrayOutput
}

// AssetArray is an input type for []AssetInput values.
type AssetArray []AssetInput

// ElementType returns the element type of this Input ([]Asset).
func (AssetArray) ElementType() reflect.Type {
	return assetArrayType
}

func (in AssetArray) ToAssetArrayOutput() AssetArrayOutput {
	return ToOutput(in).(AssetArrayOutput)
}

// AssetArrayOutput is an Output that returns []Asset values.
type AssetArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]Asset).
func (AssetArrayOutput) ElementType() reflect.Type {
	return assetArrayType
}

func (o AssetArrayOutput) ToAssetArrayOutput() AssetArrayOutput {
	return o
}

func (o AssetArrayOutput) Index(i IntInput) AssetOutput {
	return All(o, i).ApplyT(func(vs []interface{}) Asset {
		return vs[0].([]Asset)[vs[1].(int)]
	}).(AssetOutput)
}

var assetMapType = reflect.TypeOf((*map[string]Asset)(nil)).Elem()

// AssetMapInput is an input type that accepts AssetMap and AssetMapOutput values.
type AssetMapInput interface {
	Input

	ToAssetMapOutput() AssetMapOutput
}

// AssetMap is an input type for map[string]AssetInput values.
type AssetMap map[string]AssetInput

// ElementType returns the element type of this Input (map[string]Asset).
func (AssetMap) ElementType() reflect.Type {
	return assetMapType
}

func (in AssetMap) ToAssetMapOutput() AssetMapOutput {
	return ToOutput(in).(AssetMapOutput)
}

// AssetMapOutput is an Output that returns map[string]Asset values.
type AssetMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]Asset).
func (AssetMapOutput) ElementType() reflect.Type {
	return assetMapType
}

func (o AssetMapOutput) ToAssetMapOutput() AssetMapOutput {
	return o
}

func (o AssetMapOutput) MapIndex(k StringInput) AssetOutput {
	return All(o, k).ApplyT(func(vs []interface{}) Asset {
		return vs[0].(map[string]Asset)[vs[1].(string)]
	}).(AssetOutput)
}

var assetArrayMapType = reflect.TypeOf((*map[string][]Asset)(nil)).Elem()

// AssetArrayMapInput is an input type that accepts AssetArrayMap and AssetArrayMapOutput values.
type AssetArrayMapInput interface {
	Input

	ToAssetArrayMapOutput() AssetArrayMapOutput
}

// AssetArrayMap is an input type for map[string]AssetArrayInput values.
type AssetArrayMap map[string]AssetArrayInput

// ElementType returns the element type of this Input (map[string][]Asset).
func (AssetArrayMap) ElementType() reflect.Type {
	return assetArrayMapType
}

func (in AssetArrayMap) ToAssetArrayMapOutput() AssetArrayMapOutput {
	return ToOutput(in).(AssetArrayMapOutput)
}

// AssetArrayMapOutput is an Output that returns map[string][]Asset values.
type AssetArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]Asset).
func (AssetArrayMapOutput) ElementType() reflect.Type {
	return assetArrayMapType
}

func (o AssetArrayMapOutput) ToAssetArrayMapOutput() AssetArrayMapOutput {
	return o
}

func (o AssetArrayMapOutput) MapIndex(k StringInput) AssetArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []Asset {
		return vs[0].(map[string][]Asset)[vs[1].(string)]
	}).(AssetArrayOutput)
}

var assetMapArrayType = reflect.TypeOf((*[]map[string]Asset)(nil)).Elem()

// AssetMapArrayInput is an input type that accepts AssetMapArray and AssetMapArrayOutput values.
type AssetMapArrayInput interface {
	Input

	ToAssetMapArrayOutput() AssetMapArrayOutput
}

// AssetMapArray is an input type for []AssetMapInput values.
type AssetMapArray []AssetMapInput

// ElementType returns the element type of this Input ([]map[string]Asset).
func (AssetMapArray) ElementType() reflect.Type {
	return assetMapArrayType
}

func (in AssetMapArray) ToAssetMapArrayOutput() AssetMapArrayOutput {
	return ToOutput(in).(AssetMapArrayOutput)
}

// AssetMapArrayOutput is an Output that returns []map[string]Asset values.
type AssetMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]Asset).
func (AssetMapArrayOutput) ElementType() reflect.Type {
	return assetMapArrayType
}

func (o AssetMapArrayOutput) ToAssetMapArrayOutput() AssetMapArrayOutput {
	return o
}

func (o AssetMapArrayOutput) Index(i IntInput) AssetMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]Asset {
		return vs[0].([]map[string]Asset)[vs[1].(int)]
	}).(AssetMapOutput)
}

var assetOrArchiveType = reflect.TypeOf((*AssetOrArchive)(nil)).Elem()

// AssetOrArchiveInput is an input type that accepts AssetOrArchive and AssetOrArchiveOutput values.
type AssetOrArchiveInput interface {
	Input

	ToAssetOrArchiveOutput() AssetOrArchiveOutput
}

// AssetOrArchiveOutput is an Output that returns AssetOrArchive values.
type AssetOrArchiveOutput struct{ *OutputState }

// ElementType returns the element type of this Output (AssetOrArchive).
func (AssetOrArchiveOutput) ElementType() reflect.Type {
	return assetOrArchiveType
}

func (o AssetOrArchiveOutput) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return o
}

var assetOrArchiveArrayType = reflect.TypeOf((*[]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveArrayInput is an input type that accepts AssetOrArchiveArray and AssetOrArchiveArrayOutput values.
type AssetOrArchiveArrayInput interface {
	Input

	ToAssetOrArchiveArrayOutput() AssetOrArchiveArrayOutput
}

// AssetOrArchiveArray is an input type for []AssetOrArchiveInput values.
type AssetOrArchiveArray []AssetOrArchiveInput

// ElementType returns the element type of this Input ([]AssetOrArchive).
func (AssetOrArchiveArray) ElementType() reflect.Type {
	return assetOrArchiveArrayType
}

func (in AssetOrArchiveArray) ToAssetOrArchiveArrayOutput() AssetOrArchiveArrayOutput {
	return ToOutput(in).(AssetOrArchiveArrayOutput)
}

// AssetOrArchiveArrayOutput is an Output that returns []AssetOrArchive values.
type AssetOrArchiveArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]AssetOrArchive).
func (AssetOrArchiveArrayOutput) ElementType() reflect.Type {
	return assetOrArchiveArrayType
}

func (o AssetOrArchiveArrayOutput) ToAssetOrArchiveArrayOutput() AssetOrArchiveArrayOutput {
	return o
}

func (o AssetOrArchiveArrayOutput) Index(i IntInput) AssetOrArchiveOutput {
	return All(o, i).ApplyT(func(vs []interface{}) AssetOrArchive {
		return vs[0].([]AssetOrArchive)[vs[1].(int)]
	}).(AssetOrArchiveOutput)
}

var assetOrArchiveMapType = reflect.TypeOf((*map[string]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveMapInput is an input type that accepts AssetOrArchiveMap and AssetOrArchiveMapOutput values.
type AssetOrArchiveMapInput interface {
	Input

	ToAssetOrArchiveMapOutput() AssetOrArchiveMapOutput
}

// AssetOrArchiveMap is an input type for map[string]AssetOrArchiveInput values.
type AssetOrArchiveMap map[string]AssetOrArchiveInput

// ElementType returns the element type of this Input (map[string]AssetOrArchive).
func (AssetOrArchiveMap) ElementType() reflect.Type {
	return assetOrArchiveMapType
}

func (in AssetOrArchiveMap) ToAssetOrArchiveMapOutput() AssetOrArchiveMapOutput {
	return ToOutput(in).(AssetOrArchiveMapOutput)
}

// AssetOrArchiveMapOutput is an Output that returns map[string]AssetOrArchive values.
type AssetOrArchiveMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]AssetOrArchive).
func (AssetOrArchiveMapOutput) ElementType() reflect.Type {
	return assetOrArchiveMapType
}

func (o AssetOrArchiveMapOutput) ToAssetOrArchiveMapOutput() AssetOrArchiveMapOutput {
	return o
}

func (o AssetOrArchiveMapOutput) MapIndex(k StringInput) AssetOrArchiveOutput {
	return All(o, k).ApplyT(func(vs []interface{}) AssetOrArchive {
		return vs[0].(map[string]AssetOrArchive)[vs[1].(string)]
	}).(AssetOrArchiveOutput)
}

var assetOrArchiveArrayMapType = reflect.TypeOf((*map[string][]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveArrayMapInput is an input type that accepts AssetOrArchiveArrayMap and AssetOrArchiveArrayMapOutput values.
type AssetOrArchiveArrayMapInput interface {
	Input

	ToAssetOrArchiveArrayMapOutput() AssetOrArchiveArrayMapOutput
}

// AssetOrArchiveArrayMap is an input type for map[string]AssetOrArchiveArrayInput values.
type AssetOrArchiveArrayMap map[string]AssetOrArchiveArrayInput

// ElementType returns the element type of this Input (map[string][]AssetOrArchive).
func (AssetOrArchiveArrayMap) ElementType() reflect.Type {
	return assetOrArchiveArrayMapType
}

func (in AssetOrArchiveArrayMap) ToAssetOrArchiveArrayMapOutput() AssetOrArchiveArrayMapOutput {
	return ToOutput(in).(AssetOrArchiveArrayMapOutput)
}

// AssetOrArchiveArrayMapOutput is an Output that returns map[string][]AssetOrArchive values.
type AssetOrArchiveArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]AssetOrArchive).
func (AssetOrArchiveArrayMapOutput) ElementType() reflect.Type {
	return assetOrArchiveArrayMapType
}

func (o AssetOrArchiveArrayMapOutput) ToAssetOrArchiveArrayMapOutput() AssetOrArchiveArrayMapOutput {
	return o
}

func (o AssetOrArchiveArrayMapOutput) MapIndex(k StringInput) AssetOrArchiveArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []AssetOrArchive {
		return vs[0].(map[string][]AssetOrArchive)[vs[1].(string)]
	}).(AssetOrArchiveArrayOutput)
}

var assetOrArchiveMapArrayType = reflect.TypeOf((*[]map[string]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveMapArrayInput is an input type that accepts AssetOrArchiveMapArray and AssetOrArchiveMapArrayOutput values.
type AssetOrArchiveMapArrayInput interface {
	Input

	ToAssetOrArchiveMapArrayOutput() AssetOrArchiveMapArrayOutput
}

// AssetOrArchiveMapArray is an input type for []AssetOrArchiveMapInput values.
type AssetOrArchiveMapArray []AssetOrArchiveMapInput

// ElementType returns the element type of this Input ([]map[string]AssetOrArchive).
func (AssetOrArchiveMapArray) ElementType() reflect.Type {
	return assetOrArchiveMapArrayType
}

func (in AssetOrArchiveMapArray) ToAssetOrArchiveMapArrayOutput() AssetOrArchiveMapArrayOutput {
	return ToOutput(in).(AssetOrArchiveMapArrayOutput)
}

// AssetOrArchiveMapArrayOutput is an Output that returns []map[string]AssetOrArchive values.
type AssetOrArchiveMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]AssetOrArchive).
func (AssetOrArchiveMapArrayOutput) ElementType() reflect.Type {
	return assetOrArchiveMapArrayType
}

func (o AssetOrArchiveMapArrayOutput) ToAssetOrArchiveMapArrayOutput() AssetOrArchiveMapArrayOutput {
	return o
}

func (o AssetOrArchiveMapArrayOutput) Index(i IntInput) AssetOrArchiveMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]AssetOrArchive {
		return vs[0].([]map[string]AssetOrArchive)[vs[1].(int)]
	}).(AssetOrArchiveMapOutput)
}

var boolType = reflect.TypeOf((*bool)(nil)).Elem()

// BoolInput is an input type that accepts Bool and BoolOutput values.
type BoolInput interface {
	Input

	ToBoolOutput() BoolOutput
}

// Bool is an input type for bool values.
type Bool bool

// ElementType returns the element type of this Input (bool).
func (Bool) ElementType() reflect.Type {
	return boolType
}

func (in Bool) ToBoolOutput() BoolOutput {
	return ToOutput(in).(BoolOutput)
}

func (in Bool) ToBoolPtrOutput() BoolPtrOutput {
	return in.ToBoolOutput().ToBoolPtrOutput()
}

// BoolOutput is an Output that returns bool values.
type BoolOutput struct{ *OutputState }

// ElementType returns the element type of this Output (bool).
func (BoolOutput) ElementType() reflect.Type {
	return boolType
}

func (o BoolOutput) ToBoolOutput() BoolOutput {
	return o
}

func (o BoolOutput) ToBoolPtrOutput() BoolPtrOutput {
	return o.ApplyT(func(v bool) *bool {
		return &v
	}).(BoolPtrOutput)
}

var boolPtrType = reflect.TypeOf((**bool)(nil)).Elem()

// BoolPtrInput is an input type that accepts BoolPtr and BoolPtrOutput values.
type BoolPtrInput interface {
	Input

	ToBoolPtrOutput() BoolPtrOutput
}

type boolPtr bool

// BoolPtr is an input type for *bool values.
func BoolPtr(v bool) BoolPtrInput {
	return (*boolPtr)(&v)
}

// ElementType returns the element type of this Input (*bool).
func (*boolPtr) ElementType() reflect.Type {
	return boolPtrType
}

func (in *boolPtr) ToBoolPtrOutput() BoolPtrOutput {
	return ToOutput(in).(BoolPtrOutput)
}

// BoolPtrOutput is an Output that returns *bool values.
type BoolPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*bool).
func (BoolPtrOutput) ElementType() reflect.Type {
	return boolPtrType
}

func (o BoolPtrOutput) ToBoolPtrOutput() BoolPtrOutput {
	return o
}

func (o BoolPtrOutput) Elem() BoolOutput {
	return o.ApplyT(func(v *bool) bool {
		return *v
	}).(BoolOutput)
}

var boolArrayType = reflect.TypeOf((*[]bool)(nil)).Elem()

// BoolArrayInput is an input type that accepts BoolArray and BoolArrayOutput values.
type BoolArrayInput interface {
	Input

	ToBoolArrayOutput() BoolArrayOutput
}

// BoolArray is an input type for []BoolInput values.
type BoolArray []BoolInput

// ElementType returns the element type of this Input ([]bool).
func (BoolArray) ElementType() reflect.Type {
	return boolArrayType
}

func (in BoolArray) ToBoolArrayOutput() BoolArrayOutput {
	return ToOutput(in).(BoolArrayOutput)
}

// BoolArrayOutput is an Output that returns []bool values.
type BoolArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]bool).
func (BoolArrayOutput) ElementType() reflect.Type {
	return boolArrayType
}

func (o BoolArrayOutput) ToBoolArrayOutput() BoolArrayOutput {
	return o
}

func (o BoolArrayOutput) Index(i IntInput) BoolOutput {
	return All(o, i).ApplyT(func(vs []interface{}) bool {
		return vs[0].([]bool)[vs[1].(int)]
	}).(BoolOutput)
}

var boolMapType = reflect.TypeOf((*map[string]bool)(nil)).Elem()

// BoolMapInput is an input type that accepts BoolMap and BoolMapOutput values.
type BoolMapInput interface {
	Input

	ToBoolMapOutput() BoolMapOutput
}

// BoolMap is an input type for map[string]BoolInput values.
type BoolMap map[string]BoolInput

// ElementType returns the element type of this Input (map[string]bool).
func (BoolMap) ElementType() reflect.Type {
	return boolMapType
}

func (in BoolMap) ToBoolMapOutput() BoolMapOutput {
	return ToOutput(in).(BoolMapOutput)
}

// BoolMapOutput is an Output that returns map[string]bool values.
type BoolMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]bool).
func (BoolMapOutput) ElementType() reflect.Type {
	return boolMapType
}

func (o BoolMapOutput) ToBoolMapOutput() BoolMapOutput {
	return o
}

func (o BoolMapOutput) MapIndex(k StringInput) BoolOutput {
	return All(o, k).ApplyT(func(vs []interface{}) bool {
		return vs[0].(map[string]bool)[vs[1].(string)]
	}).(BoolOutput)
}

var boolArrayMapType = reflect.TypeOf((*map[string][]bool)(nil)).Elem()

// BoolArrayMapInput is an input type that accepts BoolArrayMap and BoolArrayMapOutput values.
type BoolArrayMapInput interface {
	Input

	ToBoolArrayMapOutput() BoolArrayMapOutput
}

// BoolArrayMap is an input type for map[string]BoolArrayInput values.
type BoolArrayMap map[string]BoolArrayInput

// ElementType returns the element type of this Input (map[string][]bool).
func (BoolArrayMap) ElementType() reflect.Type {
	return boolArrayMapType
}

func (in BoolArrayMap) ToBoolArrayMapOutput() BoolArrayMapOutput {
	return ToOutput(in).(BoolArrayMapOutput)
}

// BoolArrayMapOutput is an Output that returns map[string][]bool values.
type BoolArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]bool).
func (BoolArrayMapOutput) ElementType() reflect.Type {
	return boolArrayMapType
}

func (o BoolArrayMapOutput) ToBoolArrayMapOutput() BoolArrayMapOutput {
	return o
}

func (o BoolArrayMapOutput) MapIndex(k StringInput) BoolArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []bool {
		return vs[0].(map[string][]bool)[vs[1].(string)]
	}).(BoolArrayOutput)
}

var boolMapArrayType = reflect.TypeOf((*[]map[string]bool)(nil)).Elem()

// BoolMapArrayInput is an input type that accepts BoolMapArray and BoolMapArrayOutput values.
type BoolMapArrayInput interface {
	Input

	ToBoolMapArrayOutput() BoolMapArrayOutput
}

// BoolMapArray is an input type for []BoolMapInput values.
type BoolMapArray []BoolMapInput

// ElementType returns the element type of this Input ([]map[string]bool).
func (BoolMapArray) ElementType() reflect.Type {
	return boolMapArrayType
}

func (in BoolMapArray) ToBoolMapArrayOutput() BoolMapArrayOutput {
	return ToOutput(in).(BoolMapArrayOutput)
}

// BoolMapArrayOutput is an Output that returns []map[string]bool values.
type BoolMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]bool).
func (BoolMapArrayOutput) ElementType() reflect.Type {
	return boolMapArrayType
}

func (o BoolMapArrayOutput) ToBoolMapArrayOutput() BoolMapArrayOutput {
	return o
}

func (o BoolMapArrayOutput) Index(i IntInput) BoolMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]bool {
		return vs[0].([]map[string]bool)[vs[1].(int)]
	}).(BoolMapOutput)
}

var float32Type = reflect.TypeOf((*float32)(nil)).Elem()

// Float32Input is an input type that accepts Float32 and Float32Output values.
type Float32Input interface {
	Input

	ToFloat32Output() Float32Output
}

// Float32 is an input type for float32 values.
type Float32 float32

// ElementType returns the element type of this Input (float32).
func (Float32) ElementType() reflect.Type {
	return float32Type
}

func (in Float32) ToFloat32Output() Float32Output {
	return ToOutput(in).(Float32Output)
}

func (in Float32) ToFloat32PtrOutput() Float32PtrOutput {
	return in.ToFloat32Output().ToFloat32PtrOutput()
}

// Float32Output is an Output that returns float32 values.
type Float32Output struct{ *OutputState }

// ElementType returns the element type of this Output (float32).
func (Float32Output) ElementType() reflect.Type {
	return float32Type
}

func (o Float32Output) ToFloat32Output() Float32Output {
	return o
}

func (o Float32Output) ToFloat32PtrOutput() Float32PtrOutput {
	return o.ApplyT(func(v float32) *float32 {
		return &v
	}).(Float32PtrOutput)
}

var float32PtrType = reflect.TypeOf((**float32)(nil)).Elem()

// Float32PtrInput is an input type that accepts Float32Ptr and Float32PtrOutput values.
type Float32PtrInput interface {
	Input

	ToFloat32PtrOutput() Float32PtrOutput
}

type float32Ptr float32

// Float32Ptr is an input type for *float32 values.
func Float32Ptr(v float32) Float32PtrInput {
	return (*float32Ptr)(&v)
}

// ElementType returns the element type of this Input (*float32).
func (*float32Ptr) ElementType() reflect.Type {
	return float32PtrType
}

func (in *float32Ptr) ToFloat32PtrOutput() Float32PtrOutput {
	return ToOutput(in).(Float32PtrOutput)
}

// Float32PtrOutput is an Output that returns *float32 values.
type Float32PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*float32).
func (Float32PtrOutput) ElementType() reflect.Type {
	return float32PtrType
}

func (o Float32PtrOutput) ToFloat32PtrOutput() Float32PtrOutput {
	return o
}

func (o Float32PtrOutput) Elem() Float32Output {
	return o.ApplyT(func(v *float32) float32 {
		return *v
	}).(Float32Output)
}

var float32ArrayType = reflect.TypeOf((*[]float32)(nil)).Elem()

// Float32ArrayInput is an input type that accepts Float32Array and Float32ArrayOutput values.
type Float32ArrayInput interface {
	Input

	ToFloat32ArrayOutput() Float32ArrayOutput
}

// Float32Array is an input type for []Float32Input values.
type Float32Array []Float32Input

// ElementType returns the element type of this Input ([]float32).
func (Float32Array) ElementType() reflect.Type {
	return float32ArrayType
}

func (in Float32Array) ToFloat32ArrayOutput() Float32ArrayOutput {
	return ToOutput(in).(Float32ArrayOutput)
}

// Float32ArrayOutput is an Output that returns []float32 values.
type Float32ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]float32).
func (Float32ArrayOutput) ElementType() reflect.Type {
	return float32ArrayType
}

func (o Float32ArrayOutput) ToFloat32ArrayOutput() Float32ArrayOutput {
	return o
}

func (o Float32ArrayOutput) Index(i IntInput) Float32Output {
	return All(o, i).ApplyT(func(vs []interface{}) float32 {
		return vs[0].([]float32)[vs[1].(int)]
	}).(Float32Output)
}

var float32MapType = reflect.TypeOf((*map[string]float32)(nil)).Elem()

// Float32MapInput is an input type that accepts Float32Map and Float32MapOutput values.
type Float32MapInput interface {
	Input

	ToFloat32MapOutput() Float32MapOutput
}

// Float32Map is an input type for map[string]Float32Input values.
type Float32Map map[string]Float32Input

// ElementType returns the element type of this Input (map[string]float32).
func (Float32Map) ElementType() reflect.Type {
	return float32MapType
}

func (in Float32Map) ToFloat32MapOutput() Float32MapOutput {
	return ToOutput(in).(Float32MapOutput)
}

// Float32MapOutput is an Output that returns map[string]float32 values.
type Float32MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]float32).
func (Float32MapOutput) ElementType() reflect.Type {
	return float32MapType
}

func (o Float32MapOutput) ToFloat32MapOutput() Float32MapOutput {
	return o
}

func (o Float32MapOutput) MapIndex(k StringInput) Float32Output {
	return All(o, k).ApplyT(func(vs []interface{}) float32 {
		return vs[0].(map[string]float32)[vs[1].(string)]
	}).(Float32Output)
}

var float32ArrayMapType = reflect.TypeOf((*map[string][]float32)(nil)).Elem()

// Float32ArrayMapInput is an input type that accepts Float32ArrayMap and Float32ArrayMapOutput values.
type Float32ArrayMapInput interface {
	Input

	ToFloat32ArrayMapOutput() Float32ArrayMapOutput
}

// Float32ArrayMap is an input type for map[string]Float32ArrayInput values.
type Float32ArrayMap map[string]Float32ArrayInput

// ElementType returns the element type of this Input (map[string][]float32).
func (Float32ArrayMap) ElementType() reflect.Type {
	return float32ArrayMapType
}

func (in Float32ArrayMap) ToFloat32ArrayMapOutput() Float32ArrayMapOutput {
	return ToOutput(in).(Float32ArrayMapOutput)
}

// Float32ArrayMapOutput is an Output that returns map[string][]float32 values.
type Float32ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]float32).
func (Float32ArrayMapOutput) ElementType() reflect.Type {
	return float32ArrayMapType
}

func (o Float32ArrayMapOutput) ToFloat32ArrayMapOutput() Float32ArrayMapOutput {
	return o
}

func (o Float32ArrayMapOutput) MapIndex(k StringInput) Float32ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []float32 {
		return vs[0].(map[string][]float32)[vs[1].(string)]
	}).(Float32ArrayOutput)
}

var float32MapArrayType = reflect.TypeOf((*[]map[string]float32)(nil)).Elem()

// Float32MapArrayInput is an input type that accepts Float32MapArray and Float32MapArrayOutput values.
type Float32MapArrayInput interface {
	Input

	ToFloat32MapArrayOutput() Float32MapArrayOutput
}

// Float32MapArray is an input type for []Float32MapInput values.
type Float32MapArray []Float32MapInput

// ElementType returns the element type of this Input ([]map[string]float32).
func (Float32MapArray) ElementType() reflect.Type {
	return float32MapArrayType
}

func (in Float32MapArray) ToFloat32MapArrayOutput() Float32MapArrayOutput {
	return ToOutput(in).(Float32MapArrayOutput)
}

// Float32MapArrayOutput is an Output that returns []map[string]float32 values.
type Float32MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]float32).
func (Float32MapArrayOutput) ElementType() reflect.Type {
	return float32MapArrayType
}

func (o Float32MapArrayOutput) ToFloat32MapArrayOutput() Float32MapArrayOutput {
	return o
}

func (o Float32MapArrayOutput) Index(i IntInput) Float32MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]float32 {
		return vs[0].([]map[string]float32)[vs[1].(int)]
	}).(Float32MapOutput)
}

var float64Type = reflect.TypeOf((*float64)(nil)).Elem()

// Float64Input is an input type that accepts Float64 and Float64Output values.
type Float64Input interface {
	Input

	ToFloat64Output() Float64Output
}

// Float64 is an input type for float64 values.
type Float64 float64

// ElementType returns the element type of this Input (float64).
func (Float64) ElementType() reflect.Type {
	return float64Type
}

func (in Float64) ToFloat64Output() Float64Output {
	return ToOutput(in).(Float64Output)
}

func (in Float64) ToFloat64PtrOutput() Float64PtrOutput {
	return in.ToFloat64Output().ToFloat64PtrOutput()
}

// Float64Output is an Output that returns float64 values.
type Float64Output struct{ *OutputState }

// ElementType returns the element type of this Output (float64).
func (Float64Output) ElementType() reflect.Type {
	return float64Type
}

func (o Float64Output) ToFloat64Output() Float64Output {
	return o
}

func (o Float64Output) ToFloat64PtrOutput() Float64PtrOutput {
	return o.ApplyT(func(v float64) *float64 {
		return &v
	}).(Float64PtrOutput)
}

var float64PtrType = reflect.TypeOf((**float64)(nil)).Elem()

// Float64PtrInput is an input type that accepts Float64Ptr and Float64PtrOutput values.
type Float64PtrInput interface {
	Input

	ToFloat64PtrOutput() Float64PtrOutput
}

type float64Ptr float64

// Float64Ptr is an input type for *float64 values.
func Float64Ptr(v float64) Float64PtrInput {
	return (*float64Ptr)(&v)
}

// ElementType returns the element type of this Input (*float64).
func (*float64Ptr) ElementType() reflect.Type {
	return float64PtrType
}

func (in *float64Ptr) ToFloat64PtrOutput() Float64PtrOutput {
	return ToOutput(in).(Float64PtrOutput)
}

// Float64PtrOutput is an Output that returns *float64 values.
type Float64PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*float64).
func (Float64PtrOutput) ElementType() reflect.Type {
	return float64PtrType
}

func (o Float64PtrOutput) ToFloat64PtrOutput() Float64PtrOutput {
	return o
}

func (o Float64PtrOutput) Elem() Float64Output {
	return o.ApplyT(func(v *float64) float64 {
		return *v
	}).(Float64Output)
}

var float64ArrayType = reflect.TypeOf((*[]float64)(nil)).Elem()

// Float64ArrayInput is an input type that accepts Float64Array and Float64ArrayOutput values.
type Float64ArrayInput interface {
	Input

	ToFloat64ArrayOutput() Float64ArrayOutput
}

// Float64Array is an input type for []Float64Input values.
type Float64Array []Float64Input

// ElementType returns the element type of this Input ([]float64).
func (Float64Array) ElementType() reflect.Type {
	return float64ArrayType
}

func (in Float64Array) ToFloat64ArrayOutput() Float64ArrayOutput {
	return ToOutput(in).(Float64ArrayOutput)
}

// Float64ArrayOutput is an Output that returns []float64 values.
type Float64ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]float64).
func (Float64ArrayOutput) ElementType() reflect.Type {
	return float64ArrayType
}

func (o Float64ArrayOutput) ToFloat64ArrayOutput() Float64ArrayOutput {
	return o
}

func (o Float64ArrayOutput) Index(i IntInput) Float64Output {
	return All(o, i).ApplyT(func(vs []interface{}) float64 {
		return vs[0].([]float64)[vs[1].(int)]
	}).(Float64Output)
}

var float64MapType = reflect.TypeOf((*map[string]float64)(nil)).Elem()

// Float64MapInput is an input type that accepts Float64Map and Float64MapOutput values.
type Float64MapInput interface {
	Input

	ToFloat64MapOutput() Float64MapOutput
}

// Float64Map is an input type for map[string]Float64Input values.
type Float64Map map[string]Float64Input

// ElementType returns the element type of this Input (map[string]float64).
func (Float64Map) ElementType() reflect.Type {
	return float64MapType
}

func (in Float64Map) ToFloat64MapOutput() Float64MapOutput {
	return ToOutput(in).(Float64MapOutput)
}

// Float64MapOutput is an Output that returns map[string]float64 values.
type Float64MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]float64).
func (Float64MapOutput) ElementType() reflect.Type {
	return float64MapType
}

func (o Float64MapOutput) ToFloat64MapOutput() Float64MapOutput {
	return o
}

func (o Float64MapOutput) MapIndex(k StringInput) Float64Output {
	return All(o, k).ApplyT(func(vs []interface{}) float64 {
		return vs[0].(map[string]float64)[vs[1].(string)]
	}).(Float64Output)
}

var float64ArrayMapType = reflect.TypeOf((*map[string][]float64)(nil)).Elem()

// Float64ArrayMapInput is an input type that accepts Float64ArrayMap and Float64ArrayMapOutput values.
type Float64ArrayMapInput interface {
	Input

	ToFloat64ArrayMapOutput() Float64ArrayMapOutput
}

// Float64ArrayMap is an input type for map[string]Float64ArrayInput values.
type Float64ArrayMap map[string]Float64ArrayInput

// ElementType returns the element type of this Input (map[string][]float64).
func (Float64ArrayMap) ElementType() reflect.Type {
	return float64ArrayMapType
}

func (in Float64ArrayMap) ToFloat64ArrayMapOutput() Float64ArrayMapOutput {
	return ToOutput(in).(Float64ArrayMapOutput)
}

// Float64ArrayMapOutput is an Output that returns map[string][]float64 values.
type Float64ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]float64).
func (Float64ArrayMapOutput) ElementType() reflect.Type {
	return float64ArrayMapType
}

func (o Float64ArrayMapOutput) ToFloat64ArrayMapOutput() Float64ArrayMapOutput {
	return o
}

func (o Float64ArrayMapOutput) MapIndex(k StringInput) Float64ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []float64 {
		return vs[0].(map[string][]float64)[vs[1].(string)]
	}).(Float64ArrayOutput)
}

var float64MapArrayType = reflect.TypeOf((*[]map[string]float64)(nil)).Elem()

// Float64MapArrayInput is an input type that accepts Float64MapArray and Float64MapArrayOutput values.
type Float64MapArrayInput interface {
	Input

	ToFloat64MapArrayOutput() Float64MapArrayOutput
}

// Float64MapArray is an input type for []Float64MapInput values.
type Float64MapArray []Float64MapInput

// ElementType returns the element type of this Input ([]map[string]float64).
func (Float64MapArray) ElementType() reflect.Type {
	return float64MapArrayType
}

func (in Float64MapArray) ToFloat64MapArrayOutput() Float64MapArrayOutput {
	return ToOutput(in).(Float64MapArrayOutput)
}

// Float64MapArrayOutput is an Output that returns []map[string]float64 values.
type Float64MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]float64).
func (Float64MapArrayOutput) ElementType() reflect.Type {
	return float64MapArrayType
}

func (o Float64MapArrayOutput) ToFloat64MapArrayOutput() Float64MapArrayOutput {
	return o
}

func (o Float64MapArrayOutput) Index(i IntInput) Float64MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]float64 {
		return vs[0].([]map[string]float64)[vs[1].(int)]
	}).(Float64MapOutput)
}

var idType = reflect.TypeOf((*ID)(nil)).Elem()

// IDInput is an input type that accepts ID and IDOutput values.
type IDInput interface {
	Input

	ToIDOutput() IDOutput
}

// ElementType returns the element type of this Input (ID).
func (ID) ElementType() reflect.Type {
	return idType
}

func (in ID) ToIDOutput() IDOutput {
	return ToOutput(in).(IDOutput)
}

func (in ID) ToStringOutput() StringOutput {
	return in.ToIDOutput().ToStringOutput()
}

func (in ID) ToIDPtrOutput() IDPtrOutput {
	return in.ToIDOutput().ToIDPtrOutput()
}

// IDOutput is an Output that returns ID values.
type IDOutput struct{ *OutputState }

// ElementType returns the element type of this Output (ID).
func (IDOutput) ElementType() reflect.Type {
	return idType
}

func (o IDOutput) ToIDOutput() IDOutput {
	return o
}

func (o IDOutput) ToStringOutput() StringOutput {
	return o.ApplyT(func(v ID) string {
		return (string)(v)
	}).(StringOutput)
}

func (o IDOutput) ToIDPtrOutput() IDPtrOutput {
	return o.ApplyT(func(v ID) *ID {
		return &v
	}).(IDPtrOutput)
}

var iDPtrType = reflect.TypeOf((**ID)(nil)).Elem()

// IDPtrInput is an input type that accepts IDPtr and IDPtrOutput values.
type IDPtrInput interface {
	Input

	ToIDPtrOutput() IDPtrOutput
}

type idPtr ID

// IDPtr is an input type for *ID values.
func IDPtr(v ID) IDPtrInput {
	return (*idPtr)(&v)
}

// ElementType returns the element type of this Input (*ID).
func (*idPtr) ElementType() reflect.Type {
	return iDPtrType
}

func (in *idPtr) ToIDPtrOutput() IDPtrOutput {
	return ToOutput(in).(IDPtrOutput)
}

// IDPtrOutput is an Output that returns *ID values.
type IDPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*ID).
func (IDPtrOutput) ElementType() reflect.Type {
	return iDPtrType
}

func (o IDPtrOutput) ToIDPtrOutput() IDPtrOutput {
	return o
}

func (o IDPtrOutput) Elem() IDOutput {
	return o.ApplyT(func(v *ID) ID {
		return *v
	}).(IDOutput)
}

var iDArrayType = reflect.TypeOf((*[]ID)(nil)).Elem()

// IDArrayInput is an input type that accepts IDArray and IDArrayOutput values.
type IDArrayInput interface {
	Input

	ToIDArrayOutput() IDArrayOutput
}

// IDArray is an input type for []IDInput values.
type IDArray []IDInput

// ElementType returns the element type of this Input ([]ID).
func (IDArray) ElementType() reflect.Type {
	return iDArrayType
}

func (in IDArray) ToIDArrayOutput() IDArrayOutput {
	return ToOutput(in).(IDArrayOutput)
}

// IDArrayOutput is an Output that returns []ID values.
type IDArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]ID).
func (IDArrayOutput) ElementType() reflect.Type {
	return iDArrayType
}

func (o IDArrayOutput) ToIDArrayOutput() IDArrayOutput {
	return o
}

func (o IDArrayOutput) Index(i IntInput) IDOutput {
	return All(o, i).ApplyT(func(vs []interface{}) ID {
		return vs[0].([]ID)[vs[1].(int)]
	}).(IDOutput)
}

var iDMapType = reflect.TypeOf((*map[string]ID)(nil)).Elem()

// IDMapInput is an input type that accepts IDMap and IDMapOutput values.
type IDMapInput interface {
	Input

	ToIDMapOutput() IDMapOutput
}

// IDMap is an input type for map[string]IDInput values.
type IDMap map[string]IDInput

// ElementType returns the element type of this Input (map[string]ID).
func (IDMap) ElementType() reflect.Type {
	return iDMapType
}

func (in IDMap) ToIDMapOutput() IDMapOutput {
	return ToOutput(in).(IDMapOutput)
}

// IDMapOutput is an Output that returns map[string]ID values.
type IDMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]ID).
func (IDMapOutput) ElementType() reflect.Type {
	return iDMapType
}

func (o IDMapOutput) ToIDMapOutput() IDMapOutput {
	return o
}

func (o IDMapOutput) MapIndex(k StringInput) IDOutput {
	return All(o, k).ApplyT(func(vs []interface{}) ID {
		return vs[0].(map[string]ID)[vs[1].(string)]
	}).(IDOutput)
}

var iDArrayMapType = reflect.TypeOf((*map[string][]ID)(nil)).Elem()

// IDArrayMapInput is an input type that accepts IDArrayMap and IDArrayMapOutput values.
type IDArrayMapInput interface {
	Input

	ToIDArrayMapOutput() IDArrayMapOutput
}

// IDArrayMap is an input type for map[string]IDArrayInput values.
type IDArrayMap map[string]IDArrayInput

// ElementType returns the element type of this Input (map[string][]ID).
func (IDArrayMap) ElementType() reflect.Type {
	return iDArrayMapType
}

func (in IDArrayMap) ToIDArrayMapOutput() IDArrayMapOutput {
	return ToOutput(in).(IDArrayMapOutput)
}

// IDArrayMapOutput is an Output that returns map[string][]ID values.
type IDArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]ID).
func (IDArrayMapOutput) ElementType() reflect.Type {
	return iDArrayMapType
}

func (o IDArrayMapOutput) ToIDArrayMapOutput() IDArrayMapOutput {
	return o
}

func (o IDArrayMapOutput) MapIndex(k StringInput) IDArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []ID {
		return vs[0].(map[string][]ID)[vs[1].(string)]
	}).(IDArrayOutput)
}

var iDMapArrayType = reflect.TypeOf((*[]map[string]ID)(nil)).Elem()

// IDMapArrayInput is an input type that accepts IDMapArray and IDMapArrayOutput values.
type IDMapArrayInput interface {
	Input

	ToIDMapArrayOutput() IDMapArrayOutput
}

// IDMapArray is an input type for []IDMapInput values.
type IDMapArray []IDMapInput

// ElementType returns the element type of this Input ([]map[string]ID).
func (IDMapArray) ElementType() reflect.Type {
	return iDMapArrayType
}

func (in IDMapArray) ToIDMapArrayOutput() IDMapArrayOutput {
	return ToOutput(in).(IDMapArrayOutput)
}

// IDMapArrayOutput is an Output that returns []map[string]ID values.
type IDMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]ID).
func (IDMapArrayOutput) ElementType() reflect.Type {
	return iDMapArrayType
}

func (o IDMapArrayOutput) ToIDMapArrayOutput() IDMapArrayOutput {
	return o
}

func (o IDMapArrayOutput) Index(i IntInput) IDMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]ID {
		return vs[0].([]map[string]ID)[vs[1].(int)]
	}).(IDMapOutput)
}

var arrayType = reflect.TypeOf((*[]interface{})(nil)).Elem()

// ArrayInput is an input type that accepts Array and ArrayOutput values.
type ArrayInput interface {
	Input

	ToArrayOutput() ArrayOutput
}

// Array is an input type for []Input values.
type Array []Input

// ElementType returns the element type of this Input ([]interface{}).
func (Array) ElementType() reflect.Type {
	return arrayType
}

func (in Array) ToArrayOutput() ArrayOutput {
	return ToOutput(in).(ArrayOutput)
}

// ArrayOutput is an Output that returns []interface{} values.
type ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]interface{}).
func (ArrayOutput) ElementType() reflect.Type {
	return arrayType
}

func (o ArrayOutput) ToArrayOutput() ArrayOutput {
	return o
}

func (o ArrayOutput) Index(i IntInput) Output {
	return All(o, i).ApplyT(func(vs []interface{}) interface{} {
		return vs[0].([]interface{})[vs[1].(int)]
	}).(Output)
}

var mapType = reflect.TypeOf((*map[string]interface{})(nil)).Elem()

// MapInput is an input type that accepts Map and MapOutput values.
type MapInput interface {
	Input

	ToMapOutput() MapOutput
}

// Map is an input type for map[string]Input values.
type Map map[string]Input

// ElementType returns the element type of this Input (map[string]interface{}).
func (Map) ElementType() reflect.Type {
	return mapType
}

func (in Map) ToMapOutput() MapOutput {
	return ToOutput(in).(MapOutput)
}

// MapOutput is an Output that returns map[string]interface{} values.
type MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]interface{}).
func (MapOutput) ElementType() reflect.Type {
	return mapType
}

func (o MapOutput) ToMapOutput() MapOutput {
	return o
}

func (o MapOutput) MapIndex(k StringInput) Output {
	return All(o, k).ApplyT(func(vs []interface{}) interface{} {
		return vs[0].(map[string]interface{})[vs[1].(string)]
	}).(Output)
}

var arrayMapType = reflect.TypeOf((*map[string][]interface{})(nil)).Elem()

// ArrayMapInput is an input type that accepts ArrayMap and ArrayMapOutput values.
type ArrayMapInput interface {
	Input

	ToArrayMapOutput() ArrayMapOutput
}

// ArrayMap is an input type for map[string]ArrayInput values.
type ArrayMap map[string]ArrayInput

// ElementType returns the element type of this Input (map[string][]interface{}).
func (ArrayMap) ElementType() reflect.Type {
	return arrayMapType
}

func (in ArrayMap) ToArrayMapOutput() ArrayMapOutput {
	return ToOutput(in).(ArrayMapOutput)
}

// ArrayMapOutput is an Output that returns map[string][]interface{} values.
type ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]interface{}).
func (ArrayMapOutput) ElementType() reflect.Type {
	return arrayMapType
}

func (o ArrayMapOutput) ToArrayMapOutput() ArrayMapOutput {
	return o
}

func (o ArrayMapOutput) MapIndex(k StringInput) ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []interface{} {
		return vs[0].(map[string][]interface{})[vs[1].(string)]
	}).(ArrayOutput)
}

var mapArrayType = reflect.TypeOf((*[]map[string]interface{})(nil)).Elem()

// MapArrayInput is an input type that accepts MapArray and MapArrayOutput values.
type MapArrayInput interface {
	Input

	ToMapArrayOutput() MapArrayOutput
}

// MapArray is an input type for []MapInput values.
type MapArray []MapInput

// ElementType returns the element type of this Input ([]map[string]interface{}).
func (MapArray) ElementType() reflect.Type {
	return mapArrayType
}

func (in MapArray) ToMapArrayOutput() MapArrayOutput {
	return ToOutput(in).(MapArrayOutput)
}

// MapArrayOutput is an Output that returns []map[string]interface{} values.
type MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]interface{}).
func (MapArrayOutput) ElementType() reflect.Type {
	return mapArrayType
}

func (o MapArrayOutput) ToMapArrayOutput() MapArrayOutput {
	return o
}

func (o MapArrayOutput) Index(i IntInput) MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]interface{} {
		return vs[0].([]map[string]interface{})[vs[1].(int)]
	}).(MapOutput)
}

var intType = reflect.TypeOf((*int)(nil)).Elem()

// IntInput is an input type that accepts Int and IntOutput values.
type IntInput interface {
	Input

	ToIntOutput() IntOutput
}

// Int is an input type for int values.
type Int int

// ElementType returns the element type of this Input (int).
func (Int) ElementType() reflect.Type {
	return intType
}

func (in Int) ToIntOutput() IntOutput {
	return ToOutput(in).(IntOutput)
}

func (in Int) ToIntPtrOutput() IntPtrOutput {
	return in.ToIntOutput().ToIntPtrOutput()
}

// IntOutput is an Output that returns int values.
type IntOutput struct{ *OutputState }

// ElementType returns the element type of this Output (int).
func (IntOutput) ElementType() reflect.Type {
	return intType
}

func (o IntOutput) ToIntOutput() IntOutput {
	return o
}

func (o IntOutput) ToIntPtrOutput() IntPtrOutput {
	return o.ApplyT(func(v int) *int {
		return &v
	}).(IntPtrOutput)
}

var intPtrType = reflect.TypeOf((**int)(nil)).Elem()

// IntPtrInput is an input type that accepts IntPtr and IntPtrOutput values.
type IntPtrInput interface {
	Input

	ToIntPtrOutput() IntPtrOutput
}

type intPtr int

// IntPtr is an input type for *int values.
func IntPtr(v int) IntPtrInput {
	return (*intPtr)(&v)
}

// ElementType returns the element type of this Input (*int).
func (*intPtr) ElementType() reflect.Type {
	return intPtrType
}

func (in *intPtr) ToIntPtrOutput() IntPtrOutput {
	return ToOutput(in).(IntPtrOutput)
}

// IntPtrOutput is an Output that returns *int values.
type IntPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*int).
func (IntPtrOutput) ElementType() reflect.Type {
	return intPtrType
}

func (o IntPtrOutput) ToIntPtrOutput() IntPtrOutput {
	return o
}

func (o IntPtrOutput) Elem() IntOutput {
	return o.ApplyT(func(v *int) int {
		return *v
	}).(IntOutput)
}

var intArrayType = reflect.TypeOf((*[]int)(nil)).Elem()

// IntArrayInput is an input type that accepts IntArray and IntArrayOutput values.
type IntArrayInput interface {
	Input

	ToIntArrayOutput() IntArrayOutput
}

// IntArray is an input type for []IntInput values.
type IntArray []IntInput

// ElementType returns the element type of this Input ([]int).
func (IntArray) ElementType() reflect.Type {
	return intArrayType
}

func (in IntArray) ToIntArrayOutput() IntArrayOutput {
	return ToOutput(in).(IntArrayOutput)
}

// IntArrayOutput is an Output that returns []int values.
type IntArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]int).
func (IntArrayOutput) ElementType() reflect.Type {
	return intArrayType
}

func (o IntArrayOutput) ToIntArrayOutput() IntArrayOutput {
	return o
}

func (o IntArrayOutput) Index(i IntInput) IntOutput {
	return All(o, i).ApplyT(func(vs []interface{}) int {
		return vs[0].([]int)[vs[1].(int)]
	}).(IntOutput)
}

var intMapType = reflect.TypeOf((*map[string]int)(nil)).Elem()

// IntMapInput is an input type that accepts IntMap and IntMapOutput values.
type IntMapInput interface {
	Input

	ToIntMapOutput() IntMapOutput
}

// IntMap is an input type for map[string]IntInput values.
type IntMap map[string]IntInput

// ElementType returns the element type of this Input (map[string]int).
func (IntMap) ElementType() reflect.Type {
	return intMapType
}

func (in IntMap) ToIntMapOutput() IntMapOutput {
	return ToOutput(in).(IntMapOutput)
}

// IntMapOutput is an Output that returns map[string]int values.
type IntMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]int).
func (IntMapOutput) ElementType() reflect.Type {
	return intMapType
}

func (o IntMapOutput) ToIntMapOutput() IntMapOutput {
	return o
}

func (o IntMapOutput) MapIndex(k StringInput) IntOutput {
	return All(o, k).ApplyT(func(vs []interface{}) int {
		return vs[0].(map[string]int)[vs[1].(string)]
	}).(IntOutput)
}

var intArrayMapType = reflect.TypeOf((*map[string][]int)(nil)).Elem()

// IntArrayMapInput is an input type that accepts IntArrayMap and IntArrayMapOutput values.
type IntArrayMapInput interface {
	Input

	ToIntArrayMapOutput() IntArrayMapOutput
}

// IntArrayMap is an input type for map[string]IntArrayInput values.
type IntArrayMap map[string]IntArrayInput

// ElementType returns the element type of this Input (map[string][]int).
func (IntArrayMap) ElementType() reflect.Type {
	return intArrayMapType
}

func (in IntArrayMap) ToIntArrayMapOutput() IntArrayMapOutput {
	return ToOutput(in).(IntArrayMapOutput)
}

// IntArrayMapOutput is an Output that returns map[string][]int values.
type IntArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]int).
func (IntArrayMapOutput) ElementType() reflect.Type {
	return intArrayMapType
}

func (o IntArrayMapOutput) ToIntArrayMapOutput() IntArrayMapOutput {
	return o
}

func (o IntArrayMapOutput) MapIndex(k StringInput) IntArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []int {
		return vs[0].(map[string][]int)[vs[1].(string)]
	}).(IntArrayOutput)
}

var intMapArrayType = reflect.TypeOf((*[]map[string]int)(nil)).Elem()

// IntMapArrayInput is an input type that accepts IntMapArray and IntMapArrayOutput values.
type IntMapArrayInput interface {
	Input

	ToIntMapArrayOutput() IntMapArrayOutput
}

// IntMapArray is an input type for []IntMapInput values.
type IntMapArray []IntMapInput

// ElementType returns the element type of this Input ([]map[string]int).
func (IntMapArray) ElementType() reflect.Type {
	return intMapArrayType
}

func (in IntMapArray) ToIntMapArrayOutput() IntMapArrayOutput {
	return ToOutput(in).(IntMapArrayOutput)
}

// IntMapArrayOutput is an Output that returns []map[string]int values.
type IntMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]int).
func (IntMapArrayOutput) ElementType() reflect.Type {
	return intMapArrayType
}

func (o IntMapArrayOutput) ToIntMapArrayOutput() IntMapArrayOutput {
	return o
}

func (o IntMapArrayOutput) Index(i IntInput) IntMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int {
		return vs[0].([]map[string]int)[vs[1].(int)]
	}).(IntMapOutput)
}

var int16Type = reflect.TypeOf((*int16)(nil)).Elem()

// Int16Input is an input type that accepts Int16 and Int16Output values.
type Int16Input interface {
	Input

	ToInt16Output() Int16Output
}

// Int16 is an input type for int16 values.
type Int16 int16

// ElementType returns the element type of this Input (int16).
func (Int16) ElementType() reflect.Type {
	return int16Type
}

func (in Int16) ToInt16Output() Int16Output {
	return ToOutput(in).(Int16Output)
}

func (in Int16) ToInt16PtrOutput() Int16PtrOutput {
	return in.ToInt16Output().ToInt16PtrOutput()
}

// Int16Output is an Output that returns int16 values.
type Int16Output struct{ *OutputState }

// ElementType returns the element type of this Output (int16).
func (Int16Output) ElementType() reflect.Type {
	return int16Type
}

func (o Int16Output) ToInt16Output() Int16Output {
	return o
}

func (o Int16Output) ToInt16PtrOutput() Int16PtrOutput {
	return o.ApplyT(func(v int16) *int16 {
		return &v
	}).(Int16PtrOutput)
}

var int16PtrType = reflect.TypeOf((**int16)(nil)).Elem()

// Int16PtrInput is an input type that accepts Int16Ptr and Int16PtrOutput values.
type Int16PtrInput interface {
	Input

	ToInt16PtrOutput() Int16PtrOutput
}

type int16Ptr int16

// Int16Ptr is an input type for *int16 values.
func Int16Ptr(v int16) Int16PtrInput {
	return (*int16Ptr)(&v)
}

// ElementType returns the element type of this Input (*int16).
func (*int16Ptr) ElementType() reflect.Type {
	return int16PtrType
}

func (in *int16Ptr) ToInt16PtrOutput() Int16PtrOutput {
	return ToOutput(in).(Int16PtrOutput)
}

// Int16PtrOutput is an Output that returns *int16 values.
type Int16PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*int16).
func (Int16PtrOutput) ElementType() reflect.Type {
	return int16PtrType
}

func (o Int16PtrOutput) ToInt16PtrOutput() Int16PtrOutput {
	return o
}

func (o Int16PtrOutput) Elem() Int16Output {
	return o.ApplyT(func(v *int16) int16 {
		return *v
	}).(Int16Output)
}

var int16ArrayType = reflect.TypeOf((*[]int16)(nil)).Elem()

// Int16ArrayInput is an input type that accepts Int16Array and Int16ArrayOutput values.
type Int16ArrayInput interface {
	Input

	ToInt16ArrayOutput() Int16ArrayOutput
}

// Int16Array is an input type for []Int16Input values.
type Int16Array []Int16Input

// ElementType returns the element type of this Input ([]int16).
func (Int16Array) ElementType() reflect.Type {
	return int16ArrayType
}

func (in Int16Array) ToInt16ArrayOutput() Int16ArrayOutput {
	return ToOutput(in).(Int16ArrayOutput)
}

// Int16ArrayOutput is an Output that returns []int16 values.
type Int16ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]int16).
func (Int16ArrayOutput) ElementType() reflect.Type {
	return int16ArrayType
}

func (o Int16ArrayOutput) ToInt16ArrayOutput() Int16ArrayOutput {
	return o
}

func (o Int16ArrayOutput) Index(i IntInput) Int16Output {
	return All(o, i).ApplyT(func(vs []interface{}) int16 {
		return vs[0].([]int16)[vs[1].(int)]
	}).(Int16Output)
}

var int16MapType = reflect.TypeOf((*map[string]int16)(nil)).Elem()

// Int16MapInput is an input type that accepts Int16Map and Int16MapOutput values.
type Int16MapInput interface {
	Input

	ToInt16MapOutput() Int16MapOutput
}

// Int16Map is an input type for map[string]Int16Input values.
type Int16Map map[string]Int16Input

// ElementType returns the element type of this Input (map[string]int16).
func (Int16Map) ElementType() reflect.Type {
	return int16MapType
}

func (in Int16Map) ToInt16MapOutput() Int16MapOutput {
	return ToOutput(in).(Int16MapOutput)
}

// Int16MapOutput is an Output that returns map[string]int16 values.
type Int16MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]int16).
func (Int16MapOutput) ElementType() reflect.Type {
	return int16MapType
}

func (o Int16MapOutput) ToInt16MapOutput() Int16MapOutput {
	return o
}

func (o Int16MapOutput) MapIndex(k StringInput) Int16Output {
	return All(o, k).ApplyT(func(vs []interface{}) int16 {
		return vs[0].(map[string]int16)[vs[1].(string)]
	}).(Int16Output)
}

var int16ArrayMapType = reflect.TypeOf((*map[string][]int16)(nil)).Elem()

// Int16ArrayMapInput is an input type that accepts Int16ArrayMap and Int16ArrayMapOutput values.
type Int16ArrayMapInput interface {
	Input

	ToInt16ArrayMapOutput() Int16ArrayMapOutput
}

// Int16ArrayMap is an input type for map[string]Int16ArrayInput values.
type Int16ArrayMap map[string]Int16ArrayInput

// ElementType returns the element type of this Input (map[string][]int16).
func (Int16ArrayMap) ElementType() reflect.Type {
	return int16ArrayMapType
}

func (in Int16ArrayMap) ToInt16ArrayMapOutput() Int16ArrayMapOutput {
	return ToOutput(in).(Int16ArrayMapOutput)
}

// Int16ArrayMapOutput is an Output that returns map[string][]int16 values.
type Int16ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]int16).
func (Int16ArrayMapOutput) ElementType() reflect.Type {
	return int16ArrayMapType
}

func (o Int16ArrayMapOutput) ToInt16ArrayMapOutput() Int16ArrayMapOutput {
	return o
}

func (o Int16ArrayMapOutput) MapIndex(k StringInput) Int16ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []int16 {
		return vs[0].(map[string][]int16)[vs[1].(string)]
	}).(Int16ArrayOutput)
}

var int16MapArrayType = reflect.TypeOf((*[]map[string]int16)(nil)).Elem()

// Int16MapArrayInput is an input type that accepts Int16MapArray and Int16MapArrayOutput values.
type Int16MapArrayInput interface {
	Input

	ToInt16MapArrayOutput() Int16MapArrayOutput
}

// Int16MapArray is an input type for []Int16MapInput values.
type Int16MapArray []Int16MapInput

// ElementType returns the element type of this Input ([]map[string]int16).
func (Int16MapArray) ElementType() reflect.Type {
	return int16MapArrayType
}

func (in Int16MapArray) ToInt16MapArrayOutput() Int16MapArrayOutput {
	return ToOutput(in).(Int16MapArrayOutput)
}

// Int16MapArrayOutput is an Output that returns []map[string]int16 values.
type Int16MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]int16).
func (Int16MapArrayOutput) ElementType() reflect.Type {
	return int16MapArrayType
}

func (o Int16MapArrayOutput) ToInt16MapArrayOutput() Int16MapArrayOutput {
	return o
}

func (o Int16MapArrayOutput) Index(i IntInput) Int16MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int16 {
		return vs[0].([]map[string]int16)[vs[1].(int)]
	}).(Int16MapOutput)
}

var int32Type = reflect.TypeOf((*int32)(nil)).Elem()

// Int32Input is an input type that accepts Int32 and Int32Output values.
type Int32Input interface {
	Input

	ToInt32Output() Int32Output
}

// Int32 is an input type for int32 values.
type Int32 int32

// ElementType returns the element type of this Input (int32).
func (Int32) ElementType() reflect.Type {
	return int32Type
}

func (in Int32) ToInt32Output() Int32Output {
	return ToOutput(in).(Int32Output)
}

func (in Int32) ToInt32PtrOutput() Int32PtrOutput {
	return in.ToInt32Output().ToInt32PtrOutput()
}

// Int32Output is an Output that returns int32 values.
type Int32Output struct{ *OutputState }

// ElementType returns the element type of this Output (int32).
func (Int32Output) ElementType() reflect.Type {
	return int32Type
}

func (o Int32Output) ToInt32Output() Int32Output {
	return o
}

func (o Int32Output) ToInt32PtrOutput() Int32PtrOutput {
	return o.ApplyT(func(v int32) *int32 {
		return &v
	}).(Int32PtrOutput)
}

var int32PtrType = reflect.TypeOf((**int32)(nil)).Elem()

// Int32PtrInput is an input type that accepts Int32Ptr and Int32PtrOutput values.
type Int32PtrInput interface {
	Input

	ToInt32PtrOutput() Int32PtrOutput
}

type int32Ptr int32

// Int32Ptr is an input type for *int32 values.
func Int32Ptr(v int32) Int32PtrInput {
	return (*int32Ptr)(&v)
}

// ElementType returns the element type of this Input (*int32).
func (*int32Ptr) ElementType() reflect.Type {
	return int32PtrType
}

func (in *int32Ptr) ToInt32PtrOutput() Int32PtrOutput {
	return ToOutput(in).(Int32PtrOutput)
}

// Int32PtrOutput is an Output that returns *int32 values.
type Int32PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*int32).
func (Int32PtrOutput) ElementType() reflect.Type {
	return int32PtrType
}

func (o Int32PtrOutput) ToInt32PtrOutput() Int32PtrOutput {
	return o
}

func (o Int32PtrOutput) Elem() Int32Output {
	return o.ApplyT(func(v *int32) int32 {
		return *v
	}).(Int32Output)
}

var int32ArrayType = reflect.TypeOf((*[]int32)(nil)).Elem()

// Int32ArrayInput is an input type that accepts Int32Array and Int32ArrayOutput values.
type Int32ArrayInput interface {
	Input

	ToInt32ArrayOutput() Int32ArrayOutput
}

// Int32Array is an input type for []Int32Input values.
type Int32Array []Int32Input

// ElementType returns the element type of this Input ([]int32).
func (Int32Array) ElementType() reflect.Type {
	return int32ArrayType
}

func (in Int32Array) ToInt32ArrayOutput() Int32ArrayOutput {
	return ToOutput(in).(Int32ArrayOutput)
}

// Int32ArrayOutput is an Output that returns []int32 values.
type Int32ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]int32).
func (Int32ArrayOutput) ElementType() reflect.Type {
	return int32ArrayType
}

func (o Int32ArrayOutput) ToInt32ArrayOutput() Int32ArrayOutput {
	return o
}

func (o Int32ArrayOutput) Index(i IntInput) Int32Output {
	return All(o, i).ApplyT(func(vs []interface{}) int32 {
		return vs[0].([]int32)[vs[1].(int)]
	}).(Int32Output)
}

var int32MapType = reflect.TypeOf((*map[string]int32)(nil)).Elem()

// Int32MapInput is an input type that accepts Int32Map and Int32MapOutput values.
type Int32MapInput interface {
	Input

	ToInt32MapOutput() Int32MapOutput
}

// Int32Map is an input type for map[string]Int32Input values.
type Int32Map map[string]Int32Input

// ElementType returns the element type of this Input (map[string]int32).
func (Int32Map) ElementType() reflect.Type {
	return int32MapType
}

func (in Int32Map) ToInt32MapOutput() Int32MapOutput {
	return ToOutput(in).(Int32MapOutput)
}

// Int32MapOutput is an Output that returns map[string]int32 values.
type Int32MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]int32).
func (Int32MapOutput) ElementType() reflect.Type {
	return int32MapType
}

func (o Int32MapOutput) ToInt32MapOutput() Int32MapOutput {
	return o
}

func (o Int32MapOutput) MapIndex(k StringInput) Int32Output {
	return All(o, k).ApplyT(func(vs []interface{}) int32 {
		return vs[0].(map[string]int32)[vs[1].(string)]
	}).(Int32Output)
}

var int32ArrayMapType = reflect.TypeOf((*map[string][]int32)(nil)).Elem()

// Int32ArrayMapInput is an input type that accepts Int32ArrayMap and Int32ArrayMapOutput values.
type Int32ArrayMapInput interface {
	Input

	ToInt32ArrayMapOutput() Int32ArrayMapOutput
}

// Int32ArrayMap is an input type for map[string]Int32ArrayInput values.
type Int32ArrayMap map[string]Int32ArrayInput

// ElementType returns the element type of this Input (map[string][]int32).
func (Int32ArrayMap) ElementType() reflect.Type {
	return int32ArrayMapType
}

func (in Int32ArrayMap) ToInt32ArrayMapOutput() Int32ArrayMapOutput {
	return ToOutput(in).(Int32ArrayMapOutput)
}

// Int32ArrayMapOutput is an Output that returns map[string][]int32 values.
type Int32ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]int32).
func (Int32ArrayMapOutput) ElementType() reflect.Type {
	return int32ArrayMapType
}

func (o Int32ArrayMapOutput) ToInt32ArrayMapOutput() Int32ArrayMapOutput {
	return o
}

func (o Int32ArrayMapOutput) MapIndex(k StringInput) Int32ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []int32 {
		return vs[0].(map[string][]int32)[vs[1].(string)]
	}).(Int32ArrayOutput)
}

var int32MapArrayType = reflect.TypeOf((*[]map[string]int32)(nil)).Elem()

// Int32MapArrayInput is an input type that accepts Int32MapArray and Int32MapArrayOutput values.
type Int32MapArrayInput interface {
	Input

	ToInt32MapArrayOutput() Int32MapArrayOutput
}

// Int32MapArray is an input type for []Int32MapInput values.
type Int32MapArray []Int32MapInput

// ElementType returns the element type of this Input ([]map[string]int32).
func (Int32MapArray) ElementType() reflect.Type {
	return int32MapArrayType
}

func (in Int32MapArray) ToInt32MapArrayOutput() Int32MapArrayOutput {
	return ToOutput(in).(Int32MapArrayOutput)
}

// Int32MapArrayOutput is an Output that returns []map[string]int32 values.
type Int32MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]int32).
func (Int32MapArrayOutput) ElementType() reflect.Type {
	return int32MapArrayType
}

func (o Int32MapArrayOutput) ToInt32MapArrayOutput() Int32MapArrayOutput {
	return o
}

func (o Int32MapArrayOutput) Index(i IntInput) Int32MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int32 {
		return vs[0].([]map[string]int32)[vs[1].(int)]
	}).(Int32MapOutput)
}

var int64Type = reflect.TypeOf((*int64)(nil)).Elem()

// Int64Input is an input type that accepts Int64 and Int64Output values.
type Int64Input interface {
	Input

	ToInt64Output() Int64Output
}

// Int64 is an input type for int64 values.
type Int64 int64

// ElementType returns the element type of this Input (int64).
func (Int64) ElementType() reflect.Type {
	return int64Type
}

func (in Int64) ToInt64Output() Int64Output {
	return ToOutput(in).(Int64Output)
}

func (in Int64) ToInt64PtrOutput() Int64PtrOutput {
	return in.ToInt64Output().ToInt64PtrOutput()
}

// Int64Output is an Output that returns int64 values.
type Int64Output struct{ *OutputState }

// ElementType returns the element type of this Output (int64).
func (Int64Output) ElementType() reflect.Type {
	return int64Type
}

func (o Int64Output) ToInt64Output() Int64Output {
	return o
}

func (o Int64Output) ToInt64PtrOutput() Int64PtrOutput {
	return o.ApplyT(func(v int64) *int64 {
		return &v
	}).(Int64PtrOutput)
}

var int64PtrType = reflect.TypeOf((**int64)(nil)).Elem()

// Int64PtrInput is an input type that accepts Int64Ptr and Int64PtrOutput values.
type Int64PtrInput interface {
	Input

	ToInt64PtrOutput() Int64PtrOutput
}

type int64Ptr int64

// Int64Ptr is an input type for *int64 values.
func Int64Ptr(v int64) Int64PtrInput {
	return (*int64Ptr)(&v)
}

// ElementType returns the element type of this Input (*int64).
func (*int64Ptr) ElementType() reflect.Type {
	return int64PtrType
}

func (in *int64Ptr) ToInt64PtrOutput() Int64PtrOutput {
	return ToOutput(in).(Int64PtrOutput)
}

// Int64PtrOutput is an Output that returns *int64 values.
type Int64PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*int64).
func (Int64PtrOutput) ElementType() reflect.Type {
	return int64PtrType
}

func (o Int64PtrOutput) ToInt64PtrOutput() Int64PtrOutput {
	return o
}

func (o Int64PtrOutput) Elem() Int64Output {
	return o.ApplyT(func(v *int64) int64 {
		return *v
	}).(Int64Output)
}

var int64ArrayType = reflect.TypeOf((*[]int64)(nil)).Elem()

// Int64ArrayInput is an input type that accepts Int64Array and Int64ArrayOutput values.
type Int64ArrayInput interface {
	Input

	ToInt64ArrayOutput() Int64ArrayOutput
}

// Int64Array is an input type for []Int64Input values.
type Int64Array []Int64Input

// ElementType returns the element type of this Input ([]int64).
func (Int64Array) ElementType() reflect.Type {
	return int64ArrayType
}

func (in Int64Array) ToInt64ArrayOutput() Int64ArrayOutput {
	return ToOutput(in).(Int64ArrayOutput)
}

// Int64ArrayOutput is an Output that returns []int64 values.
type Int64ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]int64).
func (Int64ArrayOutput) ElementType() reflect.Type {
	return int64ArrayType
}

func (o Int64ArrayOutput) ToInt64ArrayOutput() Int64ArrayOutput {
	return o
}

func (o Int64ArrayOutput) Index(i IntInput) Int64Output {
	return All(o, i).ApplyT(func(vs []interface{}) int64 {
		return vs[0].([]int64)[vs[1].(int)]
	}).(Int64Output)
}

var int64MapType = reflect.TypeOf((*map[string]int64)(nil)).Elem()

// Int64MapInput is an input type that accepts Int64Map and Int64MapOutput values.
type Int64MapInput interface {
	Input

	ToInt64MapOutput() Int64MapOutput
}

// Int64Map is an input type for map[string]Int64Input values.
type Int64Map map[string]Int64Input

// ElementType returns the element type of this Input (map[string]int64).
func (Int64Map) ElementType() reflect.Type {
	return int64MapType
}

func (in Int64Map) ToInt64MapOutput() Int64MapOutput {
	return ToOutput(in).(Int64MapOutput)
}

// Int64MapOutput is an Output that returns map[string]int64 values.
type Int64MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]int64).
func (Int64MapOutput) ElementType() reflect.Type {
	return int64MapType
}

func (o Int64MapOutput) ToInt64MapOutput() Int64MapOutput {
	return o
}

func (o Int64MapOutput) MapIndex(k StringInput) Int64Output {
	return All(o, k).ApplyT(func(vs []interface{}) int64 {
		return vs[0].(map[string]int64)[vs[1].(string)]
	}).(Int64Output)
}

var int64ArrayMapType = reflect.TypeOf((*map[string][]int64)(nil)).Elem()

// Int64ArrayMapInput is an input type that accepts Int64ArrayMap and Int64ArrayMapOutput values.
type Int64ArrayMapInput interface {
	Input

	ToInt64ArrayMapOutput() Int64ArrayMapOutput
}

// Int64ArrayMap is an input type for map[string]Int64ArrayInput values.
type Int64ArrayMap map[string]Int64ArrayInput

// ElementType returns the element type of this Input (map[string][]int64).
func (Int64ArrayMap) ElementType() reflect.Type {
	return int64ArrayMapType
}

func (in Int64ArrayMap) ToInt64ArrayMapOutput() Int64ArrayMapOutput {
	return ToOutput(in).(Int64ArrayMapOutput)
}

// Int64ArrayMapOutput is an Output that returns map[string][]int64 values.
type Int64ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]int64).
func (Int64ArrayMapOutput) ElementType() reflect.Type {
	return int64ArrayMapType
}

func (o Int64ArrayMapOutput) ToInt64ArrayMapOutput() Int64ArrayMapOutput {
	return o
}

func (o Int64ArrayMapOutput) MapIndex(k StringInput) Int64ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []int64 {
		return vs[0].(map[string][]int64)[vs[1].(string)]
	}).(Int64ArrayOutput)
}

var int64MapArrayType = reflect.TypeOf((*[]map[string]int64)(nil)).Elem()

// Int64MapArrayInput is an input type that accepts Int64MapArray and Int64MapArrayOutput values.
type Int64MapArrayInput interface {
	Input

	ToInt64MapArrayOutput() Int64MapArrayOutput
}

// Int64MapArray is an input type for []Int64MapInput values.
type Int64MapArray []Int64MapInput

// ElementType returns the element type of this Input ([]map[string]int64).
func (Int64MapArray) ElementType() reflect.Type {
	return int64MapArrayType
}

func (in Int64MapArray) ToInt64MapArrayOutput() Int64MapArrayOutput {
	return ToOutput(in).(Int64MapArrayOutput)
}

// Int64MapArrayOutput is an Output that returns []map[string]int64 values.
type Int64MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]int64).
func (Int64MapArrayOutput) ElementType() reflect.Type {
	return int64MapArrayType
}

func (o Int64MapArrayOutput) ToInt64MapArrayOutput() Int64MapArrayOutput {
	return o
}

func (o Int64MapArrayOutput) Index(i IntInput) Int64MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int64 {
		return vs[0].([]map[string]int64)[vs[1].(int)]
	}).(Int64MapOutput)
}

var int8Type = reflect.TypeOf((*int8)(nil)).Elem()

// Int8Input is an input type that accepts Int8 and Int8Output values.
type Int8Input interface {
	Input

	ToInt8Output() Int8Output
}

// Int8 is an input type for int8 values.
type Int8 int8

// ElementType returns the element type of this Input (int8).
func (Int8) ElementType() reflect.Type {
	return int8Type
}

func (in Int8) ToInt8Output() Int8Output {
	return ToOutput(in).(Int8Output)
}

func (in Int8) ToInt8PtrOutput() Int8PtrOutput {
	return in.ToInt8Output().ToInt8PtrOutput()
}

// Int8Output is an Output that returns int8 values.
type Int8Output struct{ *OutputState }

// ElementType returns the element type of this Output (int8).
func (Int8Output) ElementType() reflect.Type {
	return int8Type
}

func (o Int8Output) ToInt8Output() Int8Output {
	return o
}

func (o Int8Output) ToInt8PtrOutput() Int8PtrOutput {
	return o.ApplyT(func(v int8) *int8 {
		return &v
	}).(Int8PtrOutput)
}

var int8PtrType = reflect.TypeOf((**int8)(nil)).Elem()

// Int8PtrInput is an input type that accepts Int8Ptr and Int8PtrOutput values.
type Int8PtrInput interface {
	Input

	ToInt8PtrOutput() Int8PtrOutput
}

type int8Ptr int8

// Int8Ptr is an input type for *int8 values.
func Int8Ptr(v int8) Int8PtrInput {
	return (*int8Ptr)(&v)
}

// ElementType returns the element type of this Input (*int8).
func (*int8Ptr) ElementType() reflect.Type {
	return int8PtrType
}

func (in *int8Ptr) ToInt8PtrOutput() Int8PtrOutput {
	return ToOutput(in).(Int8PtrOutput)
}

// Int8PtrOutput is an Output that returns *int8 values.
type Int8PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*int8).
func (Int8PtrOutput) ElementType() reflect.Type {
	return int8PtrType
}

func (o Int8PtrOutput) ToInt8PtrOutput() Int8PtrOutput {
	return o
}

func (o Int8PtrOutput) Elem() Int8Output {
	return o.ApplyT(func(v *int8) int8 {
		return *v
	}).(Int8Output)
}

var int8ArrayType = reflect.TypeOf((*[]int8)(nil)).Elem()

// Int8ArrayInput is an input type that accepts Int8Array and Int8ArrayOutput values.
type Int8ArrayInput interface {
	Input

	ToInt8ArrayOutput() Int8ArrayOutput
}

// Int8Array is an input type for []Int8Input values.
type Int8Array []Int8Input

// ElementType returns the element type of this Input ([]int8).
func (Int8Array) ElementType() reflect.Type {
	return int8ArrayType
}

func (in Int8Array) ToInt8ArrayOutput() Int8ArrayOutput {
	return ToOutput(in).(Int8ArrayOutput)
}

// Int8ArrayOutput is an Output that returns []int8 values.
type Int8ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]int8).
func (Int8ArrayOutput) ElementType() reflect.Type {
	return int8ArrayType
}

func (o Int8ArrayOutput) ToInt8ArrayOutput() Int8ArrayOutput {
	return o
}

func (o Int8ArrayOutput) Index(i IntInput) Int8Output {
	return All(o, i).ApplyT(func(vs []interface{}) int8 {
		return vs[0].([]int8)[vs[1].(int)]
	}).(Int8Output)
}

var int8MapType = reflect.TypeOf((*map[string]int8)(nil)).Elem()

// Int8MapInput is an input type that accepts Int8Map and Int8MapOutput values.
type Int8MapInput interface {
	Input

	ToInt8MapOutput() Int8MapOutput
}

// Int8Map is an input type for map[string]Int8Input values.
type Int8Map map[string]Int8Input

// ElementType returns the element type of this Input (map[string]int8).
func (Int8Map) ElementType() reflect.Type {
	return int8MapType
}

func (in Int8Map) ToInt8MapOutput() Int8MapOutput {
	return ToOutput(in).(Int8MapOutput)
}

// Int8MapOutput is an Output that returns map[string]int8 values.
type Int8MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]int8).
func (Int8MapOutput) ElementType() reflect.Type {
	return int8MapType
}

func (o Int8MapOutput) ToInt8MapOutput() Int8MapOutput {
	return o
}

func (o Int8MapOutput) MapIndex(k StringInput) Int8Output {
	return All(o, k).ApplyT(func(vs []interface{}) int8 {
		return vs[0].(map[string]int8)[vs[1].(string)]
	}).(Int8Output)
}

var int8ArrayMapType = reflect.TypeOf((*map[string][]int8)(nil)).Elem()

// Int8ArrayMapInput is an input type that accepts Int8ArrayMap and Int8ArrayMapOutput values.
type Int8ArrayMapInput interface {
	Input

	ToInt8ArrayMapOutput() Int8ArrayMapOutput
}

// Int8ArrayMap is an input type for map[string]Int8ArrayInput values.
type Int8ArrayMap map[string]Int8ArrayInput

// ElementType returns the element type of this Input (map[string][]int8).
func (Int8ArrayMap) ElementType() reflect.Type {
	return int8ArrayMapType
}

func (in Int8ArrayMap) ToInt8ArrayMapOutput() Int8ArrayMapOutput {
	return ToOutput(in).(Int8ArrayMapOutput)
}

// Int8ArrayMapOutput is an Output that returns map[string][]int8 values.
type Int8ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]int8).
func (Int8ArrayMapOutput) ElementType() reflect.Type {
	return int8ArrayMapType
}

func (o Int8ArrayMapOutput) ToInt8ArrayMapOutput() Int8ArrayMapOutput {
	return o
}

func (o Int8ArrayMapOutput) MapIndex(k StringInput) Int8ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []int8 {
		return vs[0].(map[string][]int8)[vs[1].(string)]
	}).(Int8ArrayOutput)
}

var int8MapArrayType = reflect.TypeOf((*[]map[string]int8)(nil)).Elem()

// Int8MapArrayInput is an input type that accepts Int8MapArray and Int8MapArrayOutput values.
type Int8MapArrayInput interface {
	Input

	ToInt8MapArrayOutput() Int8MapArrayOutput
}

// Int8MapArray is an input type for []Int8MapInput values.
type Int8MapArray []Int8MapInput

// ElementType returns the element type of this Input ([]map[string]int8).
func (Int8MapArray) ElementType() reflect.Type {
	return int8MapArrayType
}

func (in Int8MapArray) ToInt8MapArrayOutput() Int8MapArrayOutput {
	return ToOutput(in).(Int8MapArrayOutput)
}

// Int8MapArrayOutput is an Output that returns []map[string]int8 values.
type Int8MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]int8).
func (Int8MapArrayOutput) ElementType() reflect.Type {
	return int8MapArrayType
}

func (o Int8MapArrayOutput) ToInt8MapArrayOutput() Int8MapArrayOutput {
	return o
}

func (o Int8MapArrayOutput) Index(i IntInput) Int8MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int8 {
		return vs[0].([]map[string]int8)[vs[1].(int)]
	}).(Int8MapOutput)
}

var stringType = reflect.TypeOf((*string)(nil)).Elem()

// StringInput is an input type that accepts String and StringOutput values.
type StringInput interface {
	Input

	ToStringOutput() StringOutput
}

// String is an input type for string values.
type String string

// ElementType returns the element type of this Input (string).
func (String) ElementType() reflect.Type {
	return stringType
}

func (in String) ToStringOutput() StringOutput {
	return ToOutput(in).(StringOutput)
}

func (in String) ToStringPtrOutput() StringPtrOutput {
	return in.ToStringOutput().ToStringPtrOutput()
}

// StringOutput is an Output that returns string values.
type StringOutput struct{ *OutputState }

// ElementType returns the element type of this Output (string).
func (StringOutput) ElementType() reflect.Type {
	return stringType
}

func (o StringOutput) ToStringOutput() StringOutput {
	return o
}

func (o StringOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ApplyT(func(v string) *string {
		return &v
	}).(StringPtrOutput)
}

var stringPtrType = reflect.TypeOf((**string)(nil)).Elem()

// StringPtrInput is an input type that accepts StringPtr and StringPtrOutput values.
type StringPtrInput interface {
	Input

	ToStringPtrOutput() StringPtrOutput
}

type stringPtr string

// StringPtr is an input type for *string values.
func StringPtr(v string) StringPtrInput {
	return (*stringPtr)(&v)
}

// ElementType returns the element type of this Input (*string).
func (*stringPtr) ElementType() reflect.Type {
	return stringPtrType
}

func (in *stringPtr) ToStringPtrOutput() StringPtrOutput {
	return ToOutput(in).(StringPtrOutput)
}

// StringPtrOutput is an Output that returns *string values.
type StringPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*string).
func (StringPtrOutput) ElementType() reflect.Type {
	return stringPtrType
}

func (o StringPtrOutput) ToStringPtrOutput() StringPtrOutput {
	return o
}

func (o StringPtrOutput) Elem() StringOutput {
	return o.ApplyT(func(v *string) string {
		return *v
	}).(StringOutput)
}

var stringArrayType = reflect.TypeOf((*[]string)(nil)).Elem()

// StringArrayInput is an input type that accepts StringArray and StringArrayOutput values.
type StringArrayInput interface {
	Input

	ToStringArrayOutput() StringArrayOutput
}

// StringArray is an input type for []StringInput values.
type StringArray []StringInput

// ElementType returns the element type of this Input ([]string).
func (StringArray) ElementType() reflect.Type {
	return stringArrayType
}

func (in StringArray) ToStringArrayOutput() StringArrayOutput {
	return ToOutput(in).(StringArrayOutput)
}

// StringArrayOutput is an Output that returns []string values.
type StringArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]string).
func (StringArrayOutput) ElementType() reflect.Type {
	return stringArrayType
}

func (o StringArrayOutput) ToStringArrayOutput() StringArrayOutput {
	return o
}

func (o StringArrayOutput) Index(i IntInput) StringOutput {
	return All(o, i).ApplyT(func(vs []interface{}) string {
		return vs[0].([]string)[vs[1].(int)]
	}).(StringOutput)
}

var stringMapType = reflect.TypeOf((*map[string]string)(nil)).Elem()

// StringMapInput is an input type that accepts StringMap and StringMapOutput values.
type StringMapInput interface {
	Input

	ToStringMapOutput() StringMapOutput
}

// StringMap is an input type for map[string]StringInput values.
type StringMap map[string]StringInput

// ElementType returns the element type of this Input (map[string]string).
func (StringMap) ElementType() reflect.Type {
	return stringMapType
}

func (in StringMap) ToStringMapOutput() StringMapOutput {
	return ToOutput(in).(StringMapOutput)
}

// StringMapOutput is an Output that returns map[string]string values.
type StringMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]string).
func (StringMapOutput) ElementType() reflect.Type {
	return stringMapType
}

func (o StringMapOutput) ToStringMapOutput() StringMapOutput {
	return o
}

func (o StringMapOutput) MapIndex(k StringInput) StringOutput {
	return All(o, k).ApplyT(func(vs []interface{}) string {
		return vs[0].(map[string]string)[vs[1].(string)]
	}).(StringOutput)
}

var stringArrayMapType = reflect.TypeOf((*map[string][]string)(nil)).Elem()

// StringArrayMapInput is an input type that accepts StringArrayMap and StringArrayMapOutput values.
type StringArrayMapInput interface {
	Input

	ToStringArrayMapOutput() StringArrayMapOutput
}

// StringArrayMap is an input type for map[string]StringArrayInput values.
type StringArrayMap map[string]StringArrayInput

// ElementType returns the element type of this Input (map[string][]string).
func (StringArrayMap) ElementType() reflect.Type {
	return stringArrayMapType
}

func (in StringArrayMap) ToStringArrayMapOutput() StringArrayMapOutput {
	return ToOutput(in).(StringArrayMapOutput)
}

// StringArrayMapOutput is an Output that returns map[string][]string values.
type StringArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]string).
func (StringArrayMapOutput) ElementType() reflect.Type {
	return stringArrayMapType
}

func (o StringArrayMapOutput) ToStringArrayMapOutput() StringArrayMapOutput {
	return o
}

func (o StringArrayMapOutput) MapIndex(k StringInput) StringArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []string {
		return vs[0].(map[string][]string)[vs[1].(string)]
	}).(StringArrayOutput)
}

var stringMapArrayType = reflect.TypeOf((*[]map[string]string)(nil)).Elem()

// StringMapArrayInput is an input type that accepts StringMapArray and StringMapArrayOutput values.
type StringMapArrayInput interface {
	Input

	ToStringMapArrayOutput() StringMapArrayOutput
}

// StringMapArray is an input type for []StringMapInput values.
type StringMapArray []StringMapInput

// ElementType returns the element type of this Input ([]map[string]string).
func (StringMapArray) ElementType() reflect.Type {
	return stringMapArrayType
}

func (in StringMapArray) ToStringMapArrayOutput() StringMapArrayOutput {
	return ToOutput(in).(StringMapArrayOutput)
}

// StringMapArrayOutput is an Output that returns []map[string]string values.
type StringMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]string).
func (StringMapArrayOutput) ElementType() reflect.Type {
	return stringMapArrayType
}

func (o StringMapArrayOutput) ToStringMapArrayOutput() StringMapArrayOutput {
	return o
}

func (o StringMapArrayOutput) Index(i IntInput) StringMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]string {
		return vs[0].([]map[string]string)[vs[1].(int)]
	}).(StringMapOutput)
}

var urnType = reflect.TypeOf((*URN)(nil)).Elem()

// URNInput is an input type that accepts URN and URNOutput values.
type URNInput interface {
	Input

	ToURNOutput() URNOutput
}

// ElementType returns the element type of this Input (URN).
func (URN) ElementType() reflect.Type {
	return urnType
}

func (in URN) ToURNOutput() URNOutput {
	return ToOutput(in).(URNOutput)
}

func (in URN) ToStringOutput() StringOutput {
	return in.ToURNOutput().ToStringOutput()
}

func (in URN) ToURNPtrOutput() URNPtrOutput {
	return in.ToURNOutput().ToURNPtrOutput()
}

// URNOutput is an Output that returns URN values.
type URNOutput struct{ *OutputState }

// ElementType returns the element type of this Output (URN).
func (URNOutput) ElementType() reflect.Type {
	return urnType
}

func (o URNOutput) ToURNOutput() URNOutput {
	return o
}

func (o URNOutput) ToStringOutput() StringOutput {
	return o.ApplyT(func(v URN) string {
		return (string)(v)
	}).(StringOutput)
}

func (o URNOutput) ToURNPtrOutput() URNPtrOutput {
	return o.ApplyT(func(v URN) *URN {
		return &v
	}).(URNPtrOutput)
}

var uRNPtrType = reflect.TypeOf((**URN)(nil)).Elem()

// URNPtrInput is an input type that accepts URNPtr and URNPtrOutput values.
type URNPtrInput interface {
	Input

	ToURNPtrOutput() URNPtrOutput
}

type urnPtr URN

// URNPtr is an input type for *URN values.
func URNPtr(v URN) URNPtrInput {
	return (*urnPtr)(&v)
}

// ElementType returns the element type of this Input (*URN).
func (*urnPtr) ElementType() reflect.Type {
	return uRNPtrType
}

func (in *urnPtr) ToURNPtrOutput() URNPtrOutput {
	return ToOutput(in).(URNPtrOutput)
}

// URNPtrOutput is an Output that returns *URN values.
type URNPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*URN).
func (URNPtrOutput) ElementType() reflect.Type {
	return uRNPtrType
}

func (o URNPtrOutput) ToURNPtrOutput() URNPtrOutput {
	return o
}

func (o URNPtrOutput) Elem() URNOutput {
	return o.ApplyT(func(v *URN) URN {
		return *v
	}).(URNOutput)
}

var uRNArrayType = reflect.TypeOf((*[]URN)(nil)).Elem()

// URNArrayInput is an input type that accepts URNArray and URNArrayOutput values.
type URNArrayInput interface {
	Input

	ToURNArrayOutput() URNArrayOutput
}

// URNArray is an input type for []URNInput values.
type URNArray []URNInput

// ElementType returns the element type of this Input ([]URN).
func (URNArray) ElementType() reflect.Type {
	return uRNArrayType
}

func (in URNArray) ToURNArrayOutput() URNArrayOutput {
	return ToOutput(in).(URNArrayOutput)
}

// URNArrayOutput is an Output that returns []URN values.
type URNArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]URN).
func (URNArrayOutput) ElementType() reflect.Type {
	return uRNArrayType
}

func (o URNArrayOutput) ToURNArrayOutput() URNArrayOutput {
	return o
}

func (o URNArrayOutput) Index(i IntInput) URNOutput {
	return All(o, i).ApplyT(func(vs []interface{}) URN {
		return vs[0].([]URN)[vs[1].(int)]
	}).(URNOutput)
}

var uRNMapType = reflect.TypeOf((*map[string]URN)(nil)).Elem()

// URNMapInput is an input type that accepts URNMap and URNMapOutput values.
type URNMapInput interface {
	Input

	ToURNMapOutput() URNMapOutput
}

// URNMap is an input type for map[string]URNInput values.
type URNMap map[string]URNInput

// ElementType returns the element type of this Input (map[string]URN).
func (URNMap) ElementType() reflect.Type {
	return uRNMapType
}

func (in URNMap) ToURNMapOutput() URNMapOutput {
	return ToOutput(in).(URNMapOutput)
}

// URNMapOutput is an Output that returns map[string]URN values.
type URNMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]URN).
func (URNMapOutput) ElementType() reflect.Type {
	return uRNMapType
}

func (o URNMapOutput) ToURNMapOutput() URNMapOutput {
	return o
}

func (o URNMapOutput) MapIndex(k StringInput) URNOutput {
	return All(o, k).ApplyT(func(vs []interface{}) URN {
		return vs[0].(map[string]URN)[vs[1].(string)]
	}).(URNOutput)
}

var uRNArrayMapType = reflect.TypeOf((*map[string][]URN)(nil)).Elem()

// URNArrayMapInput is an input type that accepts URNArrayMap and URNArrayMapOutput values.
type URNArrayMapInput interface {
	Input

	ToURNArrayMapOutput() URNArrayMapOutput
}

// URNArrayMap is an input type for map[string]URNArrayInput values.
type URNArrayMap map[string]URNArrayInput

// ElementType returns the element type of this Input (map[string][]URN).
func (URNArrayMap) ElementType() reflect.Type {
	return uRNArrayMapType
}

func (in URNArrayMap) ToURNArrayMapOutput() URNArrayMapOutput {
	return ToOutput(in).(URNArrayMapOutput)
}

// URNArrayMapOutput is an Output that returns map[string][]URN values.
type URNArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]URN).
func (URNArrayMapOutput) ElementType() reflect.Type {
	return uRNArrayMapType
}

func (o URNArrayMapOutput) ToURNArrayMapOutput() URNArrayMapOutput {
	return o
}

func (o URNArrayMapOutput) MapIndex(k StringInput) URNArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []URN {
		return vs[0].(map[string][]URN)[vs[1].(string)]
	}).(URNArrayOutput)
}

var uRNMapArrayType = reflect.TypeOf((*[]map[string]URN)(nil)).Elem()

// URNMapArrayInput is an input type that accepts URNMapArray and URNMapArrayOutput values.
type URNMapArrayInput interface {
	Input

	ToURNMapArrayOutput() URNMapArrayOutput
}

// URNMapArray is an input type for []URNMapInput values.
type URNMapArray []URNMapInput

// ElementType returns the element type of this Input ([]map[string]URN).
func (URNMapArray) ElementType() reflect.Type {
	return uRNMapArrayType
}

func (in URNMapArray) ToURNMapArrayOutput() URNMapArrayOutput {
	return ToOutput(in).(URNMapArrayOutput)
}

// URNMapArrayOutput is an Output that returns []map[string]URN values.
type URNMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]URN).
func (URNMapArrayOutput) ElementType() reflect.Type {
	return uRNMapArrayType
}

func (o URNMapArrayOutput) ToURNMapArrayOutput() URNMapArrayOutput {
	return o
}

func (o URNMapArrayOutput) Index(i IntInput) URNMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]URN {
		return vs[0].([]map[string]URN)[vs[1].(int)]
	}).(URNMapOutput)
}

var uintType = reflect.TypeOf((*uint)(nil)).Elem()

// UintInput is an input type that accepts Uint and UintOutput values.
type UintInput interface {
	Input

	ToUintOutput() UintOutput
}

// Uint is an input type for uint values.
type Uint uint

// ElementType returns the element type of this Input (uint).
func (Uint) ElementType() reflect.Type {
	return uintType
}

func (in Uint) ToUintOutput() UintOutput {
	return ToOutput(in).(UintOutput)
}

func (in Uint) ToUintPtrOutput() UintPtrOutput {
	return in.ToUintOutput().ToUintPtrOutput()
}

// UintOutput is an Output that returns uint values.
type UintOutput struct{ *OutputState }

// ElementType returns the element type of this Output (uint).
func (UintOutput) ElementType() reflect.Type {
	return uintType
}

func (o UintOutput) ToUintOutput() UintOutput {
	return o
}

func (o UintOutput) ToUintPtrOutput() UintPtrOutput {
	return o.ApplyT(func(v uint) *uint {
		return &v
	}).(UintPtrOutput)
}

var uintPtrType = reflect.TypeOf((**uint)(nil)).Elem()

// UintPtrInput is an input type that accepts UintPtr and UintPtrOutput values.
type UintPtrInput interface {
	Input

	ToUintPtrOutput() UintPtrOutput
}

type uintPtr uint

// UintPtr is an input type for *uint values.
func UintPtr(v uint) UintPtrInput {
	return (*uintPtr)(&v)
}

// ElementType returns the element type of this Input (*uint).
func (*uintPtr) ElementType() reflect.Type {
	return uintPtrType
}

func (in *uintPtr) ToUintPtrOutput() UintPtrOutput {
	return ToOutput(in).(UintPtrOutput)
}

// UintPtrOutput is an Output that returns *uint values.
type UintPtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*uint).
func (UintPtrOutput) ElementType() reflect.Type {
	return uintPtrType
}

func (o UintPtrOutput) ToUintPtrOutput() UintPtrOutput {
	return o
}

func (o UintPtrOutput) Elem() UintOutput {
	return o.ApplyT(func(v *uint) uint {
		return *v
	}).(UintOutput)
}

var uintArrayType = reflect.TypeOf((*[]uint)(nil)).Elem()

// UintArrayInput is an input type that accepts UintArray and UintArrayOutput values.
type UintArrayInput interface {
	Input

	ToUintArrayOutput() UintArrayOutput
}

// UintArray is an input type for []UintInput values.
type UintArray []UintInput

// ElementType returns the element type of this Input ([]uint).
func (UintArray) ElementType() reflect.Type {
	return uintArrayType
}

func (in UintArray) ToUintArrayOutput() UintArrayOutput {
	return ToOutput(in).(UintArrayOutput)
}

// UintArrayOutput is an Output that returns []uint values.
type UintArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]uint).
func (UintArrayOutput) ElementType() reflect.Type {
	return uintArrayType
}

func (o UintArrayOutput) ToUintArrayOutput() UintArrayOutput {
	return o
}

func (o UintArrayOutput) Index(i IntInput) UintOutput {
	return All(o, i).ApplyT(func(vs []interface{}) uint {
		return vs[0].([]uint)[vs[1].(int)]
	}).(UintOutput)
}

var uintMapType = reflect.TypeOf((*map[string]uint)(nil)).Elem()

// UintMapInput is an input type that accepts UintMap and UintMapOutput values.
type UintMapInput interface {
	Input

	ToUintMapOutput() UintMapOutput
}

// UintMap is an input type for map[string]UintInput values.
type UintMap map[string]UintInput

// ElementType returns the element type of this Input (map[string]uint).
func (UintMap) ElementType() reflect.Type {
	return uintMapType
}

func (in UintMap) ToUintMapOutput() UintMapOutput {
	return ToOutput(in).(UintMapOutput)
}

// UintMapOutput is an Output that returns map[string]uint values.
type UintMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]uint).
func (UintMapOutput) ElementType() reflect.Type {
	return uintMapType
}

func (o UintMapOutput) ToUintMapOutput() UintMapOutput {
	return o
}

func (o UintMapOutput) MapIndex(k StringInput) UintOutput {
	return All(o, k).ApplyT(func(vs []interface{}) uint {
		return vs[0].(map[string]uint)[vs[1].(string)]
	}).(UintOutput)
}

var uintArrayMapType = reflect.TypeOf((*map[string][]uint)(nil)).Elem()

// UintArrayMapInput is an input type that accepts UintArrayMap and UintArrayMapOutput values.
type UintArrayMapInput interface {
	Input

	ToUintArrayMapOutput() UintArrayMapOutput
}

// UintArrayMap is an input type for map[string]UintArrayInput values.
type UintArrayMap map[string]UintArrayInput

// ElementType returns the element type of this Input (map[string][]uint).
func (UintArrayMap) ElementType() reflect.Type {
	return uintArrayMapType
}

func (in UintArrayMap) ToUintArrayMapOutput() UintArrayMapOutput {
	return ToOutput(in).(UintArrayMapOutput)
}

// UintArrayMapOutput is an Output that returns map[string][]uint values.
type UintArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]uint).
func (UintArrayMapOutput) ElementType() reflect.Type {
	return uintArrayMapType
}

func (o UintArrayMapOutput) ToUintArrayMapOutput() UintArrayMapOutput {
	return o
}

func (o UintArrayMapOutput) MapIndex(k StringInput) UintArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []uint {
		return vs[0].(map[string][]uint)[vs[1].(string)]
	}).(UintArrayOutput)
}

var uintMapArrayType = reflect.TypeOf((*[]map[string]uint)(nil)).Elem()

// UintMapArrayInput is an input type that accepts UintMapArray and UintMapArrayOutput values.
type UintMapArrayInput interface {
	Input

	ToUintMapArrayOutput() UintMapArrayOutput
}

// UintMapArray is an input type for []UintMapInput values.
type UintMapArray []UintMapInput

// ElementType returns the element type of this Input ([]map[string]uint).
func (UintMapArray) ElementType() reflect.Type {
	return uintMapArrayType
}

func (in UintMapArray) ToUintMapArrayOutput() UintMapArrayOutput {
	return ToOutput(in).(UintMapArrayOutput)
}

// UintMapArrayOutput is an Output that returns []map[string]uint values.
type UintMapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]uint).
func (UintMapArrayOutput) ElementType() reflect.Type {
	return uintMapArrayType
}

func (o UintMapArrayOutput) ToUintMapArrayOutput() UintMapArrayOutput {
	return o
}

func (o UintMapArrayOutput) Index(i IntInput) UintMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]uint {
		return vs[0].([]map[string]uint)[vs[1].(int)]
	}).(UintMapOutput)
}

var uint16Type = reflect.TypeOf((*uint16)(nil)).Elem()

// Uint16Input is an input type that accepts Uint16 and Uint16Output values.
type Uint16Input interface {
	Input

	ToUint16Output() Uint16Output
}

// Uint16 is an input type for uint16 values.
type Uint16 uint16

// ElementType returns the element type of this Input (uint16).
func (Uint16) ElementType() reflect.Type {
	return uint16Type
}

func (in Uint16) ToUint16Output() Uint16Output {
	return ToOutput(in).(Uint16Output)
}

func (in Uint16) ToUint16PtrOutput() Uint16PtrOutput {
	return in.ToUint16Output().ToUint16PtrOutput()
}

// Uint16Output is an Output that returns uint16 values.
type Uint16Output struct{ *OutputState }

// ElementType returns the element type of this Output (uint16).
func (Uint16Output) ElementType() reflect.Type {
	return uint16Type
}

func (o Uint16Output) ToUint16Output() Uint16Output {
	return o
}

func (o Uint16Output) ToUint16PtrOutput() Uint16PtrOutput {
	return o.ApplyT(func(v uint16) *uint16 {
		return &v
	}).(Uint16PtrOutput)
}

var uint16PtrType = reflect.TypeOf((**uint16)(nil)).Elem()

// Uint16PtrInput is an input type that accepts Uint16Ptr and Uint16PtrOutput values.
type Uint16PtrInput interface {
	Input

	ToUint16PtrOutput() Uint16PtrOutput
}

type uint16Ptr uint16

// Uint16Ptr is an input type for *uint16 values.
func Uint16Ptr(v uint16) Uint16PtrInput {
	return (*uint16Ptr)(&v)
}

// ElementType returns the element type of this Input (*uint16).
func (*uint16Ptr) ElementType() reflect.Type {
	return uint16PtrType
}

func (in *uint16Ptr) ToUint16PtrOutput() Uint16PtrOutput {
	return ToOutput(in).(Uint16PtrOutput)
}

// Uint16PtrOutput is an Output that returns *uint16 values.
type Uint16PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*uint16).
func (Uint16PtrOutput) ElementType() reflect.Type {
	return uint16PtrType
}

func (o Uint16PtrOutput) ToUint16PtrOutput() Uint16PtrOutput {
	return o
}

func (o Uint16PtrOutput) Elem() Uint16Output {
	return o.ApplyT(func(v *uint16) uint16 {
		return *v
	}).(Uint16Output)
}

var uint16ArrayType = reflect.TypeOf((*[]uint16)(nil)).Elem()

// Uint16ArrayInput is an input type that accepts Uint16Array and Uint16ArrayOutput values.
type Uint16ArrayInput interface {
	Input

	ToUint16ArrayOutput() Uint16ArrayOutput
}

// Uint16Array is an input type for []Uint16Input values.
type Uint16Array []Uint16Input

// ElementType returns the element type of this Input ([]uint16).
func (Uint16Array) ElementType() reflect.Type {
	return uint16ArrayType
}

func (in Uint16Array) ToUint16ArrayOutput() Uint16ArrayOutput {
	return ToOutput(in).(Uint16ArrayOutput)
}

// Uint16ArrayOutput is an Output that returns []uint16 values.
type Uint16ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]uint16).
func (Uint16ArrayOutput) ElementType() reflect.Type {
	return uint16ArrayType
}

func (o Uint16ArrayOutput) ToUint16ArrayOutput() Uint16ArrayOutput {
	return o
}

func (o Uint16ArrayOutput) Index(i IntInput) Uint16Output {
	return All(o, i).ApplyT(func(vs []interface{}) uint16 {
		return vs[0].([]uint16)[vs[1].(int)]
	}).(Uint16Output)
}

var uint16MapType = reflect.TypeOf((*map[string]uint16)(nil)).Elem()

// Uint16MapInput is an input type that accepts Uint16Map and Uint16MapOutput values.
type Uint16MapInput interface {
	Input

	ToUint16MapOutput() Uint16MapOutput
}

// Uint16Map is an input type for map[string]Uint16Input values.
type Uint16Map map[string]Uint16Input

// ElementType returns the element type of this Input (map[string]uint16).
func (Uint16Map) ElementType() reflect.Type {
	return uint16MapType
}

func (in Uint16Map) ToUint16MapOutput() Uint16MapOutput {
	return ToOutput(in).(Uint16MapOutput)
}

// Uint16MapOutput is an Output that returns map[string]uint16 values.
type Uint16MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]uint16).
func (Uint16MapOutput) ElementType() reflect.Type {
	return uint16MapType
}

func (o Uint16MapOutput) ToUint16MapOutput() Uint16MapOutput {
	return o
}

func (o Uint16MapOutput) MapIndex(k StringInput) Uint16Output {
	return All(o, k).ApplyT(func(vs []interface{}) uint16 {
		return vs[0].(map[string]uint16)[vs[1].(string)]
	}).(Uint16Output)
}

var uint16ArrayMapType = reflect.TypeOf((*map[string][]uint16)(nil)).Elem()

// Uint16ArrayMapInput is an input type that accepts Uint16ArrayMap and Uint16ArrayMapOutput values.
type Uint16ArrayMapInput interface {
	Input

	ToUint16ArrayMapOutput() Uint16ArrayMapOutput
}

// Uint16ArrayMap is an input type for map[string]Uint16ArrayInput values.
type Uint16ArrayMap map[string]Uint16ArrayInput

// ElementType returns the element type of this Input (map[string][]uint16).
func (Uint16ArrayMap) ElementType() reflect.Type {
	return uint16ArrayMapType
}

func (in Uint16ArrayMap) ToUint16ArrayMapOutput() Uint16ArrayMapOutput {
	return ToOutput(in).(Uint16ArrayMapOutput)
}

// Uint16ArrayMapOutput is an Output that returns map[string][]uint16 values.
type Uint16ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]uint16).
func (Uint16ArrayMapOutput) ElementType() reflect.Type {
	return uint16ArrayMapType
}

func (o Uint16ArrayMapOutput) ToUint16ArrayMapOutput() Uint16ArrayMapOutput {
	return o
}

func (o Uint16ArrayMapOutput) MapIndex(k StringInput) Uint16ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []uint16 {
		return vs[0].(map[string][]uint16)[vs[1].(string)]
	}).(Uint16ArrayOutput)
}

var uint16MapArrayType = reflect.TypeOf((*[]map[string]uint16)(nil)).Elem()

// Uint16MapArrayInput is an input type that accepts Uint16MapArray and Uint16MapArrayOutput values.
type Uint16MapArrayInput interface {
	Input

	ToUint16MapArrayOutput() Uint16MapArrayOutput
}

// Uint16MapArray is an input type for []Uint16MapInput values.
type Uint16MapArray []Uint16MapInput

// ElementType returns the element type of this Input ([]map[string]uint16).
func (Uint16MapArray) ElementType() reflect.Type {
	return uint16MapArrayType
}

func (in Uint16MapArray) ToUint16MapArrayOutput() Uint16MapArrayOutput {
	return ToOutput(in).(Uint16MapArrayOutput)
}

// Uint16MapArrayOutput is an Output that returns []map[string]uint16 values.
type Uint16MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]uint16).
func (Uint16MapArrayOutput) ElementType() reflect.Type {
	return uint16MapArrayType
}

func (o Uint16MapArrayOutput) ToUint16MapArrayOutput() Uint16MapArrayOutput {
	return o
}

func (o Uint16MapArrayOutput) Index(i IntInput) Uint16MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]uint16 {
		return vs[0].([]map[string]uint16)[vs[1].(int)]
	}).(Uint16MapOutput)
}

var uint32Type = reflect.TypeOf((*uint32)(nil)).Elem()

// Uint32Input is an input type that accepts Uint32 and Uint32Output values.
type Uint32Input interface {
	Input

	ToUint32Output() Uint32Output
}

// Uint32 is an input type for uint32 values.
type Uint32 uint32

// ElementType returns the element type of this Input (uint32).
func (Uint32) ElementType() reflect.Type {
	return uint32Type
}

func (in Uint32) ToUint32Output() Uint32Output {
	return ToOutput(in).(Uint32Output)
}

func (in Uint32) ToUint32PtrOutput() Uint32PtrOutput {
	return in.ToUint32Output().ToUint32PtrOutput()
}

// Uint32Output is an Output that returns uint32 values.
type Uint32Output struct{ *OutputState }

// ElementType returns the element type of this Output (uint32).
func (Uint32Output) ElementType() reflect.Type {
	return uint32Type
}

func (o Uint32Output) ToUint32Output() Uint32Output {
	return o
}

func (o Uint32Output) ToUint32PtrOutput() Uint32PtrOutput {
	return o.ApplyT(func(v uint32) *uint32 {
		return &v
	}).(Uint32PtrOutput)
}

var uint32PtrType = reflect.TypeOf((**uint32)(nil)).Elem()

// Uint32PtrInput is an input type that accepts Uint32Ptr and Uint32PtrOutput values.
type Uint32PtrInput interface {
	Input

	ToUint32PtrOutput() Uint32PtrOutput
}

type uint32Ptr uint32

// Uint32Ptr is an input type for *uint32 values.
func Uint32Ptr(v uint32) Uint32PtrInput {
	return (*uint32Ptr)(&v)
}

// ElementType returns the element type of this Input (*uint32).
func (*uint32Ptr) ElementType() reflect.Type {
	return uint32PtrType
}

func (in *uint32Ptr) ToUint32PtrOutput() Uint32PtrOutput {
	return ToOutput(in).(Uint32PtrOutput)
}

// Uint32PtrOutput is an Output that returns *uint32 values.
type Uint32PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*uint32).
func (Uint32PtrOutput) ElementType() reflect.Type {
	return uint32PtrType
}

func (o Uint32PtrOutput) ToUint32PtrOutput() Uint32PtrOutput {
	return o
}

func (o Uint32PtrOutput) Elem() Uint32Output {
	return o.ApplyT(func(v *uint32) uint32 {
		return *v
	}).(Uint32Output)
}

var uint32ArrayType = reflect.TypeOf((*[]uint32)(nil)).Elem()

// Uint32ArrayInput is an input type that accepts Uint32Array and Uint32ArrayOutput values.
type Uint32ArrayInput interface {
	Input

	ToUint32ArrayOutput() Uint32ArrayOutput
}

// Uint32Array is an input type for []Uint32Input values.
type Uint32Array []Uint32Input

// ElementType returns the element type of this Input ([]uint32).
func (Uint32Array) ElementType() reflect.Type {
	return uint32ArrayType
}

func (in Uint32Array) ToUint32ArrayOutput() Uint32ArrayOutput {
	return ToOutput(in).(Uint32ArrayOutput)
}

// Uint32ArrayOutput is an Output that returns []uint32 values.
type Uint32ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]uint32).
func (Uint32ArrayOutput) ElementType() reflect.Type {
	return uint32ArrayType
}

func (o Uint32ArrayOutput) ToUint32ArrayOutput() Uint32ArrayOutput {
	return o
}

func (o Uint32ArrayOutput) Index(i IntInput) Uint32Output {
	return All(o, i).ApplyT(func(vs []interface{}) uint32 {
		return vs[0].([]uint32)[vs[1].(int)]
	}).(Uint32Output)
}

var uint32MapType = reflect.TypeOf((*map[string]uint32)(nil)).Elem()

// Uint32MapInput is an input type that accepts Uint32Map and Uint32MapOutput values.
type Uint32MapInput interface {
	Input

	ToUint32MapOutput() Uint32MapOutput
}

// Uint32Map is an input type for map[string]Uint32Input values.
type Uint32Map map[string]Uint32Input

// ElementType returns the element type of this Input (map[string]uint32).
func (Uint32Map) ElementType() reflect.Type {
	return uint32MapType
}

func (in Uint32Map) ToUint32MapOutput() Uint32MapOutput {
	return ToOutput(in).(Uint32MapOutput)
}

// Uint32MapOutput is an Output that returns map[string]uint32 values.
type Uint32MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]uint32).
func (Uint32MapOutput) ElementType() reflect.Type {
	return uint32MapType
}

func (o Uint32MapOutput) ToUint32MapOutput() Uint32MapOutput {
	return o
}

func (o Uint32MapOutput) MapIndex(k StringInput) Uint32Output {
	return All(o, k).ApplyT(func(vs []interface{}) uint32 {
		return vs[0].(map[string]uint32)[vs[1].(string)]
	}).(Uint32Output)
}

var uint32ArrayMapType = reflect.TypeOf((*map[string][]uint32)(nil)).Elem()

// Uint32ArrayMapInput is an input type that accepts Uint32ArrayMap and Uint32ArrayMapOutput values.
type Uint32ArrayMapInput interface {
	Input

	ToUint32ArrayMapOutput() Uint32ArrayMapOutput
}

// Uint32ArrayMap is an input type for map[string]Uint32ArrayInput values.
type Uint32ArrayMap map[string]Uint32ArrayInput

// ElementType returns the element type of this Input (map[string][]uint32).
func (Uint32ArrayMap) ElementType() reflect.Type {
	return uint32ArrayMapType
}

func (in Uint32ArrayMap) ToUint32ArrayMapOutput() Uint32ArrayMapOutput {
	return ToOutput(in).(Uint32ArrayMapOutput)
}

// Uint32ArrayMapOutput is an Output that returns map[string][]uint32 values.
type Uint32ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]uint32).
func (Uint32ArrayMapOutput) ElementType() reflect.Type {
	return uint32ArrayMapType
}

func (o Uint32ArrayMapOutput) ToUint32ArrayMapOutput() Uint32ArrayMapOutput {
	return o
}

func (o Uint32ArrayMapOutput) MapIndex(k StringInput) Uint32ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []uint32 {
		return vs[0].(map[string][]uint32)[vs[1].(string)]
	}).(Uint32ArrayOutput)
}

var uint32MapArrayType = reflect.TypeOf((*[]map[string]uint32)(nil)).Elem()

// Uint32MapArrayInput is an input type that accepts Uint32MapArray and Uint32MapArrayOutput values.
type Uint32MapArrayInput interface {
	Input

	ToUint32MapArrayOutput() Uint32MapArrayOutput
}

// Uint32MapArray is an input type for []Uint32MapInput values.
type Uint32MapArray []Uint32MapInput

// ElementType returns the element type of this Input ([]map[string]uint32).
func (Uint32MapArray) ElementType() reflect.Type {
	return uint32MapArrayType
}

func (in Uint32MapArray) ToUint32MapArrayOutput() Uint32MapArrayOutput {
	return ToOutput(in).(Uint32MapArrayOutput)
}

// Uint32MapArrayOutput is an Output that returns []map[string]uint32 values.
type Uint32MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]uint32).
func (Uint32MapArrayOutput) ElementType() reflect.Type {
	return uint32MapArrayType
}

func (o Uint32MapArrayOutput) ToUint32MapArrayOutput() Uint32MapArrayOutput {
	return o
}

func (o Uint32MapArrayOutput) Index(i IntInput) Uint32MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]uint32 {
		return vs[0].([]map[string]uint32)[vs[1].(int)]
	}).(Uint32MapOutput)
}

var uint64Type = reflect.TypeOf((*uint64)(nil)).Elem()

// Uint64Input is an input type that accepts Uint64 and Uint64Output values.
type Uint64Input interface {
	Input

	ToUint64Output() Uint64Output
}

// Uint64 is an input type for uint64 values.
type Uint64 uint64

// ElementType returns the element type of this Input (uint64).
func (Uint64) ElementType() reflect.Type {
	return uint64Type
}

func (in Uint64) ToUint64Output() Uint64Output {
	return ToOutput(in).(Uint64Output)
}

func (in Uint64) ToUint64PtrOutput() Uint64PtrOutput {
	return in.ToUint64Output().ToUint64PtrOutput()
}

// Uint64Output is an Output that returns uint64 values.
type Uint64Output struct{ *OutputState }

// ElementType returns the element type of this Output (uint64).
func (Uint64Output) ElementType() reflect.Type {
	return uint64Type
}

func (o Uint64Output) ToUint64Output() Uint64Output {
	return o
}

func (o Uint64Output) ToUint64PtrOutput() Uint64PtrOutput {
	return o.ApplyT(func(v uint64) *uint64 {
		return &v
	}).(Uint64PtrOutput)
}

var uint64PtrType = reflect.TypeOf((**uint64)(nil)).Elem()

// Uint64PtrInput is an input type that accepts Uint64Ptr and Uint64PtrOutput values.
type Uint64PtrInput interface {
	Input

	ToUint64PtrOutput() Uint64PtrOutput
}

type uint64Ptr uint64

// Uint64Ptr is an input type for *uint64 values.
func Uint64Ptr(v uint64) Uint64PtrInput {
	return (*uint64Ptr)(&v)
}

// ElementType returns the element type of this Input (*uint64).
func (*uint64Ptr) ElementType() reflect.Type {
	return uint64PtrType
}

func (in *uint64Ptr) ToUint64PtrOutput() Uint64PtrOutput {
	return ToOutput(in).(Uint64PtrOutput)
}

// Uint64PtrOutput is an Output that returns *uint64 values.
type Uint64PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*uint64).
func (Uint64PtrOutput) ElementType() reflect.Type {
	return uint64PtrType
}

func (o Uint64PtrOutput) ToUint64PtrOutput() Uint64PtrOutput {
	return o
}

func (o Uint64PtrOutput) Elem() Uint64Output {
	return o.ApplyT(func(v *uint64) uint64 {
		return *v
	}).(Uint64Output)
}

var uint64ArrayType = reflect.TypeOf((*[]uint64)(nil)).Elem()

// Uint64ArrayInput is an input type that accepts Uint64Array and Uint64ArrayOutput values.
type Uint64ArrayInput interface {
	Input

	ToUint64ArrayOutput() Uint64ArrayOutput
}

// Uint64Array is an input type for []Uint64Input values.
type Uint64Array []Uint64Input

// ElementType returns the element type of this Input ([]uint64).
func (Uint64Array) ElementType() reflect.Type {
	return uint64ArrayType
}

func (in Uint64Array) ToUint64ArrayOutput() Uint64ArrayOutput {
	return ToOutput(in).(Uint64ArrayOutput)
}

// Uint64ArrayOutput is an Output that returns []uint64 values.
type Uint64ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]uint64).
func (Uint64ArrayOutput) ElementType() reflect.Type {
	return uint64ArrayType
}

func (o Uint64ArrayOutput) ToUint64ArrayOutput() Uint64ArrayOutput {
	return o
}

func (o Uint64ArrayOutput) Index(i IntInput) Uint64Output {
	return All(o, i).ApplyT(func(vs []interface{}) uint64 {
		return vs[0].([]uint64)[vs[1].(int)]
	}).(Uint64Output)
}

var uint64MapType = reflect.TypeOf((*map[string]uint64)(nil)).Elem()

// Uint64MapInput is an input type that accepts Uint64Map and Uint64MapOutput values.
type Uint64MapInput interface {
	Input

	ToUint64MapOutput() Uint64MapOutput
}

// Uint64Map is an input type for map[string]Uint64Input values.
type Uint64Map map[string]Uint64Input

// ElementType returns the element type of this Input (map[string]uint64).
func (Uint64Map) ElementType() reflect.Type {
	return uint64MapType
}

func (in Uint64Map) ToUint64MapOutput() Uint64MapOutput {
	return ToOutput(in).(Uint64MapOutput)
}

// Uint64MapOutput is an Output that returns map[string]uint64 values.
type Uint64MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]uint64).
func (Uint64MapOutput) ElementType() reflect.Type {
	return uint64MapType
}

func (o Uint64MapOutput) ToUint64MapOutput() Uint64MapOutput {
	return o
}

func (o Uint64MapOutput) MapIndex(k StringInput) Uint64Output {
	return All(o, k).ApplyT(func(vs []interface{}) uint64 {
		return vs[0].(map[string]uint64)[vs[1].(string)]
	}).(Uint64Output)
}

var uint64ArrayMapType = reflect.TypeOf((*map[string][]uint64)(nil)).Elem()

// Uint64ArrayMapInput is an input type that accepts Uint64ArrayMap and Uint64ArrayMapOutput values.
type Uint64ArrayMapInput interface {
	Input

	ToUint64ArrayMapOutput() Uint64ArrayMapOutput
}

// Uint64ArrayMap is an input type for map[string]Uint64ArrayInput values.
type Uint64ArrayMap map[string]Uint64ArrayInput

// ElementType returns the element type of this Input (map[string][]uint64).
func (Uint64ArrayMap) ElementType() reflect.Type {
	return uint64ArrayMapType
}

func (in Uint64ArrayMap) ToUint64ArrayMapOutput() Uint64ArrayMapOutput {
	return ToOutput(in).(Uint64ArrayMapOutput)
}

// Uint64ArrayMapOutput is an Output that returns map[string][]uint64 values.
type Uint64ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]uint64).
func (Uint64ArrayMapOutput) ElementType() reflect.Type {
	return uint64ArrayMapType
}

func (o Uint64ArrayMapOutput) ToUint64ArrayMapOutput() Uint64ArrayMapOutput {
	return o
}

func (o Uint64ArrayMapOutput) MapIndex(k StringInput) Uint64ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []uint64 {
		return vs[0].(map[string][]uint64)[vs[1].(string)]
	}).(Uint64ArrayOutput)
}

var uint64MapArrayType = reflect.TypeOf((*[]map[string]uint64)(nil)).Elem()

// Uint64MapArrayInput is an input type that accepts Uint64MapArray and Uint64MapArrayOutput values.
type Uint64MapArrayInput interface {
	Input

	ToUint64MapArrayOutput() Uint64MapArrayOutput
}

// Uint64MapArray is an input type for []Uint64MapInput values.
type Uint64MapArray []Uint64MapInput

// ElementType returns the element type of this Input ([]map[string]uint64).
func (Uint64MapArray) ElementType() reflect.Type {
	return uint64MapArrayType
}

func (in Uint64MapArray) ToUint64MapArrayOutput() Uint64MapArrayOutput {
	return ToOutput(in).(Uint64MapArrayOutput)
}

// Uint64MapArrayOutput is an Output that returns []map[string]uint64 values.
type Uint64MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]uint64).
func (Uint64MapArrayOutput) ElementType() reflect.Type {
	return uint64MapArrayType
}

func (o Uint64MapArrayOutput) ToUint64MapArrayOutput() Uint64MapArrayOutput {
	return o
}

func (o Uint64MapArrayOutput) Index(i IntInput) Uint64MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]uint64 {
		return vs[0].([]map[string]uint64)[vs[1].(int)]
	}).(Uint64MapOutput)
}

var uint8Type = reflect.TypeOf((*uint8)(nil)).Elem()

// Uint8Input is an input type that accepts Uint8 and Uint8Output values.
type Uint8Input interface {
	Input

	ToUint8Output() Uint8Output
}

// Uint8 is an input type for uint8 values.
type Uint8 uint8

// ElementType returns the element type of this Input (uint8).
func (Uint8) ElementType() reflect.Type {
	return uint8Type
}

func (in Uint8) ToUint8Output() Uint8Output {
	return ToOutput(in).(Uint8Output)
}

func (in Uint8) ToUint8PtrOutput() Uint8PtrOutput {
	return in.ToUint8Output().ToUint8PtrOutput()
}

// Uint8Output is an Output that returns uint8 values.
type Uint8Output struct{ *OutputState }

// ElementType returns the element type of this Output (uint8).
func (Uint8Output) ElementType() reflect.Type {
	return uint8Type
}

func (o Uint8Output) ToUint8Output() Uint8Output {
	return o
}

func (o Uint8Output) ToUint8PtrOutput() Uint8PtrOutput {
	return o.ApplyT(func(v uint8) *uint8 {
		return &v
	}).(Uint8PtrOutput)
}

var uint8PtrType = reflect.TypeOf((**uint8)(nil)).Elem()

// Uint8PtrInput is an input type that accepts Uint8Ptr and Uint8PtrOutput values.
type Uint8PtrInput interface {
	Input

	ToUint8PtrOutput() Uint8PtrOutput
}

type uint8Ptr uint8

// Uint8Ptr is an input type for *uint8 values.
func Uint8Ptr(v uint8) Uint8PtrInput {
	return (*uint8Ptr)(&v)
}

// ElementType returns the element type of this Input (*uint8).
func (*uint8Ptr) ElementType() reflect.Type {
	return uint8PtrType
}

func (in *uint8Ptr) ToUint8PtrOutput() Uint8PtrOutput {
	return ToOutput(in).(Uint8PtrOutput)
}

// Uint8PtrOutput is an Output that returns *uint8 values.
type Uint8PtrOutput struct{ *OutputState }

// ElementType returns the element type of this Output (*uint8).
func (Uint8PtrOutput) ElementType() reflect.Type {
	return uint8PtrType
}

func (o Uint8PtrOutput) ToUint8PtrOutput() Uint8PtrOutput {
	return o
}

func (o Uint8PtrOutput) Elem() Uint8Output {
	return o.ApplyT(func(v *uint8) uint8 {
		return *v
	}).(Uint8Output)
}

var uint8ArrayType = reflect.TypeOf((*[]uint8)(nil)).Elem()

// Uint8ArrayInput is an input type that accepts Uint8Array and Uint8ArrayOutput values.
type Uint8ArrayInput interface {
	Input

	ToUint8ArrayOutput() Uint8ArrayOutput
}

// Uint8Array is an input type for []Uint8Input values.
type Uint8Array []Uint8Input

// ElementType returns the element type of this Input ([]uint8).
func (Uint8Array) ElementType() reflect.Type {
	return uint8ArrayType
}

func (in Uint8Array) ToUint8ArrayOutput() Uint8ArrayOutput {
	return ToOutput(in).(Uint8ArrayOutput)
}

// Uint8ArrayOutput is an Output that returns []uint8 values.
type Uint8ArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]uint8).
func (Uint8ArrayOutput) ElementType() reflect.Type {
	return uint8ArrayType
}

func (o Uint8ArrayOutput) ToUint8ArrayOutput() Uint8ArrayOutput {
	return o
}

func (o Uint8ArrayOutput) Index(i IntInput) Uint8Output {
	return All(o, i).ApplyT(func(vs []interface{}) uint8 {
		return vs[0].([]uint8)[vs[1].(int)]
	}).(Uint8Output)
}

var uint8MapType = reflect.TypeOf((*map[string]uint8)(nil)).Elem()

// Uint8MapInput is an input type that accepts Uint8Map and Uint8MapOutput values.
type Uint8MapInput interface {
	Input

	ToUint8MapOutput() Uint8MapOutput
}

// Uint8Map is an input type for map[string]Uint8Input values.
type Uint8Map map[string]Uint8Input

// ElementType returns the element type of this Input (map[string]uint8).
func (Uint8Map) ElementType() reflect.Type {
	return uint8MapType
}

func (in Uint8Map) ToUint8MapOutput() Uint8MapOutput {
	return ToOutput(in).(Uint8MapOutput)
}

// Uint8MapOutput is an Output that returns map[string]uint8 values.
type Uint8MapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]uint8).
func (Uint8MapOutput) ElementType() reflect.Type {
	return uint8MapType
}

func (o Uint8MapOutput) ToUint8MapOutput() Uint8MapOutput {
	return o
}

func (o Uint8MapOutput) MapIndex(k StringInput) Uint8Output {
	return All(o, k).ApplyT(func(vs []interface{}) uint8 {
		return vs[0].(map[string]uint8)[vs[1].(string)]
	}).(Uint8Output)
}

var uint8ArrayMapType = reflect.TypeOf((*map[string][]uint8)(nil)).Elem()

// Uint8ArrayMapInput is an input type that accepts Uint8ArrayMap and Uint8ArrayMapOutput values.
type Uint8ArrayMapInput interface {
	Input

	ToUint8ArrayMapOutput() Uint8ArrayMapOutput
}

// Uint8ArrayMap is an input type for map[string]Uint8ArrayInput values.
type Uint8ArrayMap map[string]Uint8ArrayInput

// ElementType returns the element type of this Input (map[string][]uint8).
func (Uint8ArrayMap) ElementType() reflect.Type {
	return uint8ArrayMapType
}

func (in Uint8ArrayMap) ToUint8ArrayMapOutput() Uint8ArrayMapOutput {
	return ToOutput(in).(Uint8ArrayMapOutput)
}

// Uint8ArrayMapOutput is an Output that returns map[string][]uint8 values.
type Uint8ArrayMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string][]uint8).
func (Uint8ArrayMapOutput) ElementType() reflect.Type {
	return uint8ArrayMapType
}

func (o Uint8ArrayMapOutput) ToUint8ArrayMapOutput() Uint8ArrayMapOutput {
	return o
}

func (o Uint8ArrayMapOutput) MapIndex(k StringInput) Uint8ArrayOutput {
	return All(o, k).ApplyT(func(vs []interface{}) []uint8 {
		return vs[0].(map[string][]uint8)[vs[1].(string)]
	}).(Uint8ArrayOutput)
}

var uint8MapArrayType = reflect.TypeOf((*[]map[string]uint8)(nil)).Elem()

// Uint8MapArrayInput is an input type that accepts Uint8MapArray and Uint8MapArrayOutput values.
type Uint8MapArrayInput interface {
	Input

	ToUint8MapArrayOutput() Uint8MapArrayOutput
}

// Uint8MapArray is an input type for []Uint8MapInput values.
type Uint8MapArray []Uint8MapInput

// ElementType returns the element type of this Input ([]map[string]uint8).
func (Uint8MapArray) ElementType() reflect.Type {
	return uint8MapArrayType
}

func (in Uint8MapArray) ToUint8MapArrayOutput() Uint8MapArrayOutput {
	return ToOutput(in).(Uint8MapArrayOutput)
}

// Uint8MapArrayOutput is an Output that returns []map[string]uint8 values.
type Uint8MapArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([]map[string]uint8).
func (Uint8MapArrayOutput) ElementType() reflect.Type {
	return uint8MapArrayType
}

func (o Uint8MapArrayOutput) ToUint8MapArrayOutput() Uint8MapArrayOutput {
	return o
}

func (o Uint8MapArrayOutput) Index(i IntInput) Uint8MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]uint8 {
		return vs[0].([]map[string]uint8)[vs[1].(int)]
	}).(Uint8MapOutput)
}

func getResolvedValue(input Input) (reflect.Value, bool) {
	switch input := input.(type) {
	case *asset, *archive:
		return reflect.ValueOf(input), true
	default:
		return reflect.Value{}, false
	}
}

func init() {
	RegisterOutputType(ArchiveOutput{})
	RegisterOutputType(ArchiveArrayOutput{})
	RegisterOutputType(ArchiveMapOutput{})
	RegisterOutputType(ArchiveArrayMapOutput{})
	RegisterOutputType(ArchiveMapArrayOutput{})
	RegisterOutputType(AssetOutput{})
	RegisterOutputType(AssetArrayOutput{})
	RegisterOutputType(AssetMapOutput{})
	RegisterOutputType(AssetArrayMapOutput{})
	RegisterOutputType(AssetMapArrayOutput{})
	RegisterOutputType(AssetOrArchiveOutput{})
	RegisterOutputType(AssetOrArchiveArrayOutput{})
	RegisterOutputType(AssetOrArchiveMapOutput{})
	RegisterOutputType(AssetOrArchiveArrayMapOutput{})
	RegisterOutputType(AssetOrArchiveMapArrayOutput{})
	RegisterOutputType(BoolOutput{})
	RegisterOutputType(BoolPtrOutput{})
	RegisterOutputType(BoolArrayOutput{})
	RegisterOutputType(BoolMapOutput{})
	RegisterOutputType(BoolArrayMapOutput{})
	RegisterOutputType(BoolMapArrayOutput{})
	RegisterOutputType(Float32Output{})
	RegisterOutputType(Float32PtrOutput{})
	RegisterOutputType(Float32ArrayOutput{})
	RegisterOutputType(Float32MapOutput{})
	RegisterOutputType(Float32ArrayMapOutput{})
	RegisterOutputType(Float32MapArrayOutput{})
	RegisterOutputType(Float64Output{})
	RegisterOutputType(Float64PtrOutput{})
	RegisterOutputType(Float64ArrayOutput{})
	RegisterOutputType(Float64MapOutput{})
	RegisterOutputType(Float64ArrayMapOutput{})
	RegisterOutputType(Float64MapArrayOutput{})
	RegisterOutputType(IDOutput{})
	RegisterOutputType(IDPtrOutput{})
	RegisterOutputType(IDArrayOutput{})
	RegisterOutputType(IDMapOutput{})
	RegisterOutputType(IDArrayMapOutput{})
	RegisterOutputType(IDMapArrayOutput{})
	RegisterOutputType(ArrayOutput{})
	RegisterOutputType(MapOutput{})
	RegisterOutputType(ArrayMapOutput{})
	RegisterOutputType(MapArrayOutput{})
	RegisterOutputType(IntOutput{})
	RegisterOutputType(IntPtrOutput{})
	RegisterOutputType(IntArrayOutput{})
	RegisterOutputType(IntMapOutput{})
	RegisterOutputType(IntArrayMapOutput{})
	RegisterOutputType(IntMapArrayOutput{})
	RegisterOutputType(Int16Output{})
	RegisterOutputType(Int16PtrOutput{})
	RegisterOutputType(Int16ArrayOutput{})
	RegisterOutputType(Int16MapOutput{})
	RegisterOutputType(Int16ArrayMapOutput{})
	RegisterOutputType(Int16MapArrayOutput{})
	RegisterOutputType(Int32Output{})
	RegisterOutputType(Int32PtrOutput{})
	RegisterOutputType(Int32ArrayOutput{})
	RegisterOutputType(Int32MapOutput{})
	RegisterOutputType(Int32ArrayMapOutput{})
	RegisterOutputType(Int32MapArrayOutput{})
	RegisterOutputType(Int64Output{})
	RegisterOutputType(Int64PtrOutput{})
	RegisterOutputType(Int64ArrayOutput{})
	RegisterOutputType(Int64MapOutput{})
	RegisterOutputType(Int64ArrayMapOutput{})
	RegisterOutputType(Int64MapArrayOutput{})
	RegisterOutputType(Int8Output{})
	RegisterOutputType(Int8PtrOutput{})
	RegisterOutputType(Int8ArrayOutput{})
	RegisterOutputType(Int8MapOutput{})
	RegisterOutputType(Int8ArrayMapOutput{})
	RegisterOutputType(Int8MapArrayOutput{})
	RegisterOutputType(StringOutput{})
	RegisterOutputType(StringPtrOutput{})
	RegisterOutputType(StringArrayOutput{})
	RegisterOutputType(StringMapOutput{})
	RegisterOutputType(StringArrayMapOutput{})
	RegisterOutputType(StringMapArrayOutput{})
	RegisterOutputType(URNOutput{})
	RegisterOutputType(URNPtrOutput{})
	RegisterOutputType(URNArrayOutput{})
	RegisterOutputType(URNMapOutput{})
	RegisterOutputType(URNArrayMapOutput{})
	RegisterOutputType(URNMapArrayOutput{})
	RegisterOutputType(UintOutput{})
	RegisterOutputType(UintPtrOutput{})
	RegisterOutputType(UintArrayOutput{})
	RegisterOutputType(UintMapOutput{})
	RegisterOutputType(UintArrayMapOutput{})
	RegisterOutputType(UintMapArrayOutput{})
	RegisterOutputType(Uint16Output{})
	RegisterOutputType(Uint16PtrOutput{})
	RegisterOutputType(Uint16ArrayOutput{})
	RegisterOutputType(Uint16MapOutput{})
	RegisterOutputType(Uint16ArrayMapOutput{})
	RegisterOutputType(Uint16MapArrayOutput{})
	RegisterOutputType(Uint32Output{})
	RegisterOutputType(Uint32PtrOutput{})
	RegisterOutputType(Uint32ArrayOutput{})
	RegisterOutputType(Uint32MapOutput{})
	RegisterOutputType(Uint32ArrayMapOutput{})
	RegisterOutputType(Uint32MapArrayOutput{})
	RegisterOutputType(Uint64Output{})
	RegisterOutputType(Uint64PtrOutput{})
	RegisterOutputType(Uint64ArrayOutput{})
	RegisterOutputType(Uint64MapOutput{})
	RegisterOutputType(Uint64ArrayMapOutput{})
	RegisterOutputType(Uint64MapArrayOutput{})
	RegisterOutputType(Uint8Output{})
	RegisterOutputType(Uint8PtrOutput{})
	RegisterOutputType(Uint8ArrayOutput{})
	RegisterOutputType(Uint8MapOutput{})
	RegisterOutputType(Uint8ArrayMapOutput{})
	RegisterOutputType(Uint8MapArrayOutput{})
}
