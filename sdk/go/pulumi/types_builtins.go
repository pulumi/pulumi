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
	"context"
	"reflect"
)

// ApplyArchive is like ApplyT, but returns a ArchiveOutput.
func (o *OutputState) ApplyArchive(applier interface{}) ArchiveOutput {
	return o.ApplyT(applier).(ArchiveOutput)
}

// ApplyArchiveWithContext is like ApplyTWithContext, but returns a ArchiveOutput.
func (o *OutputState) ApplyArchiveWithContext(ctx context.Context, applier interface{}) ArchiveOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveOutput)
}

// ApplyArchiveArray is like ApplyT, but returns a ArchiveArrayOutput.
func (o *OutputState) ApplyArchiveArray(applier interface{}) ArchiveArrayOutput {
	return o.ApplyT(applier).(ArchiveArrayOutput)
}

// ApplyArchiveArrayWithContext is like ApplyTWithContext, but returns a ArchiveArrayOutput.
func (o *OutputState) ApplyArchiveArrayWithContext(ctx context.Context, applier interface{}) ArchiveArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveArrayOutput)
}

// ApplyArchiveMap is like ApplyT, but returns a ArchiveMapOutput.
func (o *OutputState) ApplyArchiveMap(applier interface{}) ArchiveMapOutput {
	return o.ApplyT(applier).(ArchiveMapOutput)
}

// ApplyArchiveMapWithContext is like ApplyTWithContext, but returns a ArchiveMapOutput.
func (o *OutputState) ApplyArchiveMapWithContext(ctx context.Context, applier interface{}) ArchiveMapOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveMapOutput)
}

// ApplyArchiveArrayMap is like ApplyT, but returns a ArchiveArrayMapOutput.
func (o *OutputState) ApplyArchiveArrayMap(applier interface{}) ArchiveArrayMapOutput {
	return o.ApplyT(applier).(ArchiveArrayMapOutput)
}

// ApplyArchiveArrayMapWithContext is like ApplyTWithContext, but returns a ArchiveArrayMapOutput.
func (o *OutputState) ApplyArchiveArrayMapWithContext(ctx context.Context, applier interface{}) ArchiveArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveArrayMapOutput)
}

// ApplyArchiveMapArray is like ApplyT, but returns a ArchiveMapArrayOutput.
func (o *OutputState) ApplyArchiveMapArray(applier interface{}) ArchiveMapArrayOutput {
	return o.ApplyT(applier).(ArchiveMapArrayOutput)
}

// ApplyArchiveMapArrayWithContext is like ApplyTWithContext, but returns a ArchiveMapArrayOutput.
func (o *OutputState) ApplyArchiveMapArrayWithContext(ctx context.Context, applier interface{}) ArchiveMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveMapArrayOutput)
}

// ApplyArchiveMapMap is like ApplyT, but returns a ArchiveMapMapOutput.
func (o *OutputState) ApplyArchiveMapMap(applier interface{}) ArchiveMapMapOutput {
	return o.ApplyT(applier).(ArchiveMapMapOutput)
}

// ApplyArchiveMapMapWithContext is like ApplyTWithContext, but returns a ArchiveMapMapOutput.
func (o *OutputState) ApplyArchiveMapMapWithContext(ctx context.Context, applier interface{}) ArchiveMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveMapMapOutput)
}

// ApplyArchiveArrayArray is like ApplyT, but returns a ArchiveArrayArrayOutput.
func (o *OutputState) ApplyArchiveArrayArray(applier interface{}) ArchiveArrayArrayOutput {
	return o.ApplyT(applier).(ArchiveArrayArrayOutput)
}

// ApplyArchiveArrayArrayWithContext is like ApplyTWithContext, but returns a ArchiveArrayArrayOutput.
func (o *OutputState) ApplyArchiveArrayArrayWithContext(ctx context.Context, applier interface{}) ArchiveArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(ArchiveArrayArrayOutput)
}

// ApplyAsset is like ApplyT, but returns a AssetOutput.
func (o *OutputState) ApplyAsset(applier interface{}) AssetOutput {
	return o.ApplyT(applier).(AssetOutput)
}

// ApplyAssetWithContext is like ApplyTWithContext, but returns a AssetOutput.
func (o *OutputState) ApplyAssetWithContext(ctx context.Context, applier interface{}) AssetOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOutput)
}

// ApplyAssetArray is like ApplyT, but returns a AssetArrayOutput.
func (o *OutputState) ApplyAssetArray(applier interface{}) AssetArrayOutput {
	return o.ApplyT(applier).(AssetArrayOutput)
}

// ApplyAssetArrayWithContext is like ApplyTWithContext, but returns a AssetArrayOutput.
func (o *OutputState) ApplyAssetArrayWithContext(ctx context.Context, applier interface{}) AssetArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetArrayOutput)
}

// ApplyAssetMap is like ApplyT, but returns a AssetMapOutput.
func (o *OutputState) ApplyAssetMap(applier interface{}) AssetMapOutput {
	return o.ApplyT(applier).(AssetMapOutput)
}

// ApplyAssetMapWithContext is like ApplyTWithContext, but returns a AssetMapOutput.
func (o *OutputState) ApplyAssetMapWithContext(ctx context.Context, applier interface{}) AssetMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetMapOutput)
}

// ApplyAssetArrayMap is like ApplyT, but returns a AssetArrayMapOutput.
func (o *OutputState) ApplyAssetArrayMap(applier interface{}) AssetArrayMapOutput {
	return o.ApplyT(applier).(AssetArrayMapOutput)
}

// ApplyAssetArrayMapWithContext is like ApplyTWithContext, but returns a AssetArrayMapOutput.
func (o *OutputState) ApplyAssetArrayMapWithContext(ctx context.Context, applier interface{}) AssetArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetArrayMapOutput)
}

// ApplyAssetMapArray is like ApplyT, but returns a AssetMapArrayOutput.
func (o *OutputState) ApplyAssetMapArray(applier interface{}) AssetMapArrayOutput {
	return o.ApplyT(applier).(AssetMapArrayOutput)
}

// ApplyAssetMapArrayWithContext is like ApplyTWithContext, but returns a AssetMapArrayOutput.
func (o *OutputState) ApplyAssetMapArrayWithContext(ctx context.Context, applier interface{}) AssetMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetMapArrayOutput)
}

// ApplyAssetMapMap is like ApplyT, but returns a AssetMapMapOutput.
func (o *OutputState) ApplyAssetMapMap(applier interface{}) AssetMapMapOutput {
	return o.ApplyT(applier).(AssetMapMapOutput)
}

// ApplyAssetMapMapWithContext is like ApplyTWithContext, but returns a AssetMapMapOutput.
func (o *OutputState) ApplyAssetMapMapWithContext(ctx context.Context, applier interface{}) AssetMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetMapMapOutput)
}

// ApplyAssetArrayArray is like ApplyT, but returns a AssetArrayArrayOutput.
func (o *OutputState) ApplyAssetArrayArray(applier interface{}) AssetArrayArrayOutput {
	return o.ApplyT(applier).(AssetArrayArrayOutput)
}

// ApplyAssetArrayArrayWithContext is like ApplyTWithContext, but returns a AssetArrayArrayOutput.
func (o *OutputState) ApplyAssetArrayArrayWithContext(ctx context.Context, applier interface{}) AssetArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetArrayArrayOutput)
}

// ApplyAssetOrArchive is like ApplyT, but returns a AssetOrArchiveOutput.
func (o *OutputState) ApplyAssetOrArchive(applier interface{}) AssetOrArchiveOutput {
	return o.ApplyT(applier).(AssetOrArchiveOutput)
}

// ApplyAssetOrArchiveWithContext is like ApplyTWithContext, but returns a AssetOrArchiveOutput.
func (o *OutputState) ApplyAssetOrArchiveWithContext(ctx context.Context, applier interface{}) AssetOrArchiveOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveOutput)
}

// ApplyAssetOrArchiveArray is like ApplyT, but returns a AssetOrArchiveArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveArray(applier interface{}) AssetOrArchiveArrayOutput {
	return o.ApplyT(applier).(AssetOrArchiveArrayOutput)
}

// ApplyAssetOrArchiveArrayWithContext is like ApplyTWithContext, but returns a AssetOrArchiveArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayWithContext(ctx context.Context, applier interface{}) AssetOrArchiveArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveArrayOutput)
}

// ApplyAssetOrArchiveMap is like ApplyT, but returns a AssetOrArchiveMapOutput.
func (o *OutputState) ApplyAssetOrArchiveMap(applier interface{}) AssetOrArchiveMapOutput {
	return o.ApplyT(applier).(AssetOrArchiveMapOutput)
}

// ApplyAssetOrArchiveMapWithContext is like ApplyTWithContext, but returns a AssetOrArchiveMapOutput.
func (o *OutputState) ApplyAssetOrArchiveMapWithContext(ctx context.Context, applier interface{}) AssetOrArchiveMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveMapOutput)
}

// ApplyAssetOrArchiveArrayMap is like ApplyT, but returns a AssetOrArchiveArrayMapOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayMap(applier interface{}) AssetOrArchiveArrayMapOutput {
	return o.ApplyT(applier).(AssetOrArchiveArrayMapOutput)
}

// ApplyAssetOrArchiveArrayMapWithContext is like ApplyTWithContext, but returns a AssetOrArchiveArrayMapOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayMapWithContext(ctx context.Context, applier interface{}) AssetOrArchiveArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveArrayMapOutput)
}

// ApplyAssetOrArchiveMapArray is like ApplyT, but returns a AssetOrArchiveMapArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveMapArray(applier interface{}) AssetOrArchiveMapArrayOutput {
	return o.ApplyT(applier).(AssetOrArchiveMapArrayOutput)
}

// ApplyAssetOrArchiveMapArrayWithContext is like ApplyTWithContext, but returns a AssetOrArchiveMapArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveMapArrayWithContext(ctx context.Context, applier interface{}) AssetOrArchiveMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveMapArrayOutput)
}

// ApplyAssetOrArchiveMapMap is like ApplyT, but returns a AssetOrArchiveMapMapOutput.
func (o *OutputState) ApplyAssetOrArchiveMapMap(applier interface{}) AssetOrArchiveMapMapOutput {
	return o.ApplyT(applier).(AssetOrArchiveMapMapOutput)
}

// ApplyAssetOrArchiveMapMapWithContext is like ApplyTWithContext, but returns a AssetOrArchiveMapMapOutput.
func (o *OutputState) ApplyAssetOrArchiveMapMapWithContext(ctx context.Context, applier interface{}) AssetOrArchiveMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveMapMapOutput)
}

// ApplyAssetOrArchiveArrayArray is like ApplyT, but returns a AssetOrArchiveArrayArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayArray(applier interface{}) AssetOrArchiveArrayArrayOutput {
	return o.ApplyT(applier).(AssetOrArchiveArrayArrayOutput)
}

// ApplyAssetOrArchiveArrayArrayWithContext is like ApplyTWithContext, but returns a AssetOrArchiveArrayArrayOutput.
func (o *OutputState) ApplyAssetOrArchiveArrayArrayWithContext(ctx context.Context, applier interface{}) AssetOrArchiveArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(AssetOrArchiveArrayArrayOutput)
}

// ApplyBool is like ApplyT, but returns a BoolOutput.
func (o *OutputState) ApplyBool(applier interface{}) BoolOutput {
	return o.ApplyT(applier).(BoolOutput)
}

// ApplyBoolWithContext is like ApplyTWithContext, but returns a BoolOutput.
func (o *OutputState) ApplyBoolWithContext(ctx context.Context, applier interface{}) BoolOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolOutput)
}

// ApplyBoolPtr is like ApplyT, but returns a BoolPtrOutput.
func (o *OutputState) ApplyBoolPtr(applier interface{}) BoolPtrOutput {
	return o.ApplyT(applier).(BoolPtrOutput)
}

// ApplyBoolPtrWithContext is like ApplyTWithContext, but returns a BoolPtrOutput.
func (o *OutputState) ApplyBoolPtrWithContext(ctx context.Context, applier interface{}) BoolPtrOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolPtrOutput)
}

// ApplyBoolArray is like ApplyT, but returns a BoolArrayOutput.
func (o *OutputState) ApplyBoolArray(applier interface{}) BoolArrayOutput {
	return o.ApplyT(applier).(BoolArrayOutput)
}

// ApplyBoolArrayWithContext is like ApplyTWithContext, but returns a BoolArrayOutput.
func (o *OutputState) ApplyBoolArrayWithContext(ctx context.Context, applier interface{}) BoolArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolArrayOutput)
}

// ApplyBoolMap is like ApplyT, but returns a BoolMapOutput.
func (o *OutputState) ApplyBoolMap(applier interface{}) BoolMapOutput {
	return o.ApplyT(applier).(BoolMapOutput)
}

// ApplyBoolMapWithContext is like ApplyTWithContext, but returns a BoolMapOutput.
func (o *OutputState) ApplyBoolMapWithContext(ctx context.Context, applier interface{}) BoolMapOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolMapOutput)
}

// ApplyBoolArrayMap is like ApplyT, but returns a BoolArrayMapOutput.
func (o *OutputState) ApplyBoolArrayMap(applier interface{}) BoolArrayMapOutput {
	return o.ApplyT(applier).(BoolArrayMapOutput)
}

// ApplyBoolArrayMapWithContext is like ApplyTWithContext, but returns a BoolArrayMapOutput.
func (o *OutputState) ApplyBoolArrayMapWithContext(ctx context.Context, applier interface{}) BoolArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolArrayMapOutput)
}

// ApplyBoolMapArray is like ApplyT, but returns a BoolMapArrayOutput.
func (o *OutputState) ApplyBoolMapArray(applier interface{}) BoolMapArrayOutput {
	return o.ApplyT(applier).(BoolMapArrayOutput)
}

// ApplyBoolMapArrayWithContext is like ApplyTWithContext, but returns a BoolMapArrayOutput.
func (o *OutputState) ApplyBoolMapArrayWithContext(ctx context.Context, applier interface{}) BoolMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolMapArrayOutput)
}

// ApplyBoolMapMap is like ApplyT, but returns a BoolMapMapOutput.
func (o *OutputState) ApplyBoolMapMap(applier interface{}) BoolMapMapOutput {
	return o.ApplyT(applier).(BoolMapMapOutput)
}

// ApplyBoolMapMapWithContext is like ApplyTWithContext, but returns a BoolMapMapOutput.
func (o *OutputState) ApplyBoolMapMapWithContext(ctx context.Context, applier interface{}) BoolMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolMapMapOutput)
}

// ApplyBoolArrayArray is like ApplyT, but returns a BoolArrayArrayOutput.
func (o *OutputState) ApplyBoolArrayArray(applier interface{}) BoolArrayArrayOutput {
	return o.ApplyT(applier).(BoolArrayArrayOutput)
}

// ApplyBoolArrayArrayWithContext is like ApplyTWithContext, but returns a BoolArrayArrayOutput.
func (o *OutputState) ApplyBoolArrayArrayWithContext(ctx context.Context, applier interface{}) BoolArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(BoolArrayArrayOutput)
}

// ApplyFloat64 is like ApplyT, but returns a Float64Output.
func (o *OutputState) ApplyFloat64(applier interface{}) Float64Output {
	return o.ApplyT(applier).(Float64Output)
}

// ApplyFloat64WithContext is like ApplyTWithContext, but returns a Float64Output.
func (o *OutputState) ApplyFloat64WithContext(ctx context.Context, applier interface{}) Float64Output {
	return o.ApplyTWithContext(ctx, applier).(Float64Output)
}

// ApplyFloat64Ptr is like ApplyT, but returns a Float64PtrOutput.
func (o *OutputState) ApplyFloat64Ptr(applier interface{}) Float64PtrOutput {
	return o.ApplyT(applier).(Float64PtrOutput)
}

// ApplyFloat64PtrWithContext is like ApplyTWithContext, but returns a Float64PtrOutput.
func (o *OutputState) ApplyFloat64PtrWithContext(ctx context.Context, applier interface{}) Float64PtrOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64PtrOutput)
}

// ApplyFloat64Array is like ApplyT, but returns a Float64ArrayOutput.
func (o *OutputState) ApplyFloat64Array(applier interface{}) Float64ArrayOutput {
	return o.ApplyT(applier).(Float64ArrayOutput)
}

// ApplyFloat64ArrayWithContext is like ApplyTWithContext, but returns a Float64ArrayOutput.
func (o *OutputState) ApplyFloat64ArrayWithContext(ctx context.Context, applier interface{}) Float64ArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64ArrayOutput)
}

// ApplyFloat64Map is like ApplyT, but returns a Float64MapOutput.
func (o *OutputState) ApplyFloat64Map(applier interface{}) Float64MapOutput {
	return o.ApplyT(applier).(Float64MapOutput)
}

// ApplyFloat64MapWithContext is like ApplyTWithContext, but returns a Float64MapOutput.
func (o *OutputState) ApplyFloat64MapWithContext(ctx context.Context, applier interface{}) Float64MapOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64MapOutput)
}

// ApplyFloat64ArrayMap is like ApplyT, but returns a Float64ArrayMapOutput.
func (o *OutputState) ApplyFloat64ArrayMap(applier interface{}) Float64ArrayMapOutput {
	return o.ApplyT(applier).(Float64ArrayMapOutput)
}

// ApplyFloat64ArrayMapWithContext is like ApplyTWithContext, but returns a Float64ArrayMapOutput.
func (o *OutputState) ApplyFloat64ArrayMapWithContext(ctx context.Context, applier interface{}) Float64ArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64ArrayMapOutput)
}

// ApplyFloat64MapArray is like ApplyT, but returns a Float64MapArrayOutput.
func (o *OutputState) ApplyFloat64MapArray(applier interface{}) Float64MapArrayOutput {
	return o.ApplyT(applier).(Float64MapArrayOutput)
}

// ApplyFloat64MapArrayWithContext is like ApplyTWithContext, but returns a Float64MapArrayOutput.
func (o *OutputState) ApplyFloat64MapArrayWithContext(ctx context.Context, applier interface{}) Float64MapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64MapArrayOutput)
}

// ApplyFloat64MapMap is like ApplyT, but returns a Float64MapMapOutput.
func (o *OutputState) ApplyFloat64MapMap(applier interface{}) Float64MapMapOutput {
	return o.ApplyT(applier).(Float64MapMapOutput)
}

// ApplyFloat64MapMapWithContext is like ApplyTWithContext, but returns a Float64MapMapOutput.
func (o *OutputState) ApplyFloat64MapMapWithContext(ctx context.Context, applier interface{}) Float64MapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64MapMapOutput)
}

// ApplyFloat64ArrayArray is like ApplyT, but returns a Float64ArrayArrayOutput.
func (o *OutputState) ApplyFloat64ArrayArray(applier interface{}) Float64ArrayArrayOutput {
	return o.ApplyT(applier).(Float64ArrayArrayOutput)
}

// ApplyFloat64ArrayArrayWithContext is like ApplyTWithContext, but returns a Float64ArrayArrayOutput.
func (o *OutputState) ApplyFloat64ArrayArrayWithContext(ctx context.Context, applier interface{}) Float64ArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(Float64ArrayArrayOutput)
}

// ApplyID is like ApplyT, but returns a IDOutput.
func (o *OutputState) ApplyID(applier interface{}) IDOutput {
	return o.ApplyT(applier).(IDOutput)
}

// ApplyIDWithContext is like ApplyTWithContext, but returns a IDOutput.
func (o *OutputState) ApplyIDWithContext(ctx context.Context, applier interface{}) IDOutput {
	return o.ApplyTWithContext(ctx, applier).(IDOutput)
}

// ApplyIDPtr is like ApplyT, but returns a IDPtrOutput.
func (o *OutputState) ApplyIDPtr(applier interface{}) IDPtrOutput {
	return o.ApplyT(applier).(IDPtrOutput)
}

// ApplyIDPtrWithContext is like ApplyTWithContext, but returns a IDPtrOutput.
func (o *OutputState) ApplyIDPtrWithContext(ctx context.Context, applier interface{}) IDPtrOutput {
	return o.ApplyTWithContext(ctx, applier).(IDPtrOutput)
}

// ApplyIDArray is like ApplyT, but returns a IDArrayOutput.
func (o *OutputState) ApplyIDArray(applier interface{}) IDArrayOutput {
	return o.ApplyT(applier).(IDArrayOutput)
}

// ApplyIDArrayWithContext is like ApplyTWithContext, but returns a IDArrayOutput.
func (o *OutputState) ApplyIDArrayWithContext(ctx context.Context, applier interface{}) IDArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IDArrayOutput)
}

// ApplyIDMap is like ApplyT, but returns a IDMapOutput.
func (o *OutputState) ApplyIDMap(applier interface{}) IDMapOutput {
	return o.ApplyT(applier).(IDMapOutput)
}

// ApplyIDMapWithContext is like ApplyTWithContext, but returns a IDMapOutput.
func (o *OutputState) ApplyIDMapWithContext(ctx context.Context, applier interface{}) IDMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IDMapOutput)
}

// ApplyIDArrayMap is like ApplyT, but returns a IDArrayMapOutput.
func (o *OutputState) ApplyIDArrayMap(applier interface{}) IDArrayMapOutput {
	return o.ApplyT(applier).(IDArrayMapOutput)
}

// ApplyIDArrayMapWithContext is like ApplyTWithContext, but returns a IDArrayMapOutput.
func (o *OutputState) ApplyIDArrayMapWithContext(ctx context.Context, applier interface{}) IDArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IDArrayMapOutput)
}

// ApplyIDMapArray is like ApplyT, but returns a IDMapArrayOutput.
func (o *OutputState) ApplyIDMapArray(applier interface{}) IDMapArrayOutput {
	return o.ApplyT(applier).(IDMapArrayOutput)
}

// ApplyIDMapArrayWithContext is like ApplyTWithContext, but returns a IDMapArrayOutput.
func (o *OutputState) ApplyIDMapArrayWithContext(ctx context.Context, applier interface{}) IDMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IDMapArrayOutput)
}

// ApplyIDMapMap is like ApplyT, but returns a IDMapMapOutput.
func (o *OutputState) ApplyIDMapMap(applier interface{}) IDMapMapOutput {
	return o.ApplyT(applier).(IDMapMapOutput)
}

// ApplyIDMapMapWithContext is like ApplyTWithContext, but returns a IDMapMapOutput.
func (o *OutputState) ApplyIDMapMapWithContext(ctx context.Context, applier interface{}) IDMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IDMapMapOutput)
}

// ApplyIDArrayArray is like ApplyT, but returns a IDArrayArrayOutput.
func (o *OutputState) ApplyIDArrayArray(applier interface{}) IDArrayArrayOutput {
	return o.ApplyT(applier).(IDArrayArrayOutput)
}

// ApplyIDArrayArrayWithContext is like ApplyTWithContext, but returns a IDArrayArrayOutput.
func (o *OutputState) ApplyIDArrayArrayWithContext(ctx context.Context, applier interface{}) IDArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IDArrayArrayOutput)
}

// ApplyArray is like ApplyT, but returns a ArrayOutput.
func (o *OutputState) ApplyArray(applier interface{}) ArrayOutput {
	return o.ApplyT(applier).(ArrayOutput)
}

// ApplyArrayWithContext is like ApplyTWithContext, but returns a ArrayOutput.
func (o *OutputState) ApplyArrayWithContext(ctx context.Context, applier interface{}) ArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(ArrayOutput)
}

// ApplyMap is like ApplyT, but returns a MapOutput.
func (o *OutputState) ApplyMap(applier interface{}) MapOutput {
	return o.ApplyT(applier).(MapOutput)
}

// ApplyMapWithContext is like ApplyTWithContext, but returns a MapOutput.
func (o *OutputState) ApplyMapWithContext(ctx context.Context, applier interface{}) MapOutput {
	return o.ApplyTWithContext(ctx, applier).(MapOutput)
}

// ApplyArrayMap is like ApplyT, but returns a ArrayMapOutput.
func (o *OutputState) ApplyArrayMap(applier interface{}) ArrayMapOutput {
	return o.ApplyT(applier).(ArrayMapOutput)
}

// ApplyArrayMapWithContext is like ApplyTWithContext, but returns a ArrayMapOutput.
func (o *OutputState) ApplyArrayMapWithContext(ctx context.Context, applier interface{}) ArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(ArrayMapOutput)
}

// ApplyMapArray is like ApplyT, but returns a MapArrayOutput.
func (o *OutputState) ApplyMapArray(applier interface{}) MapArrayOutput {
	return o.ApplyT(applier).(MapArrayOutput)
}

// ApplyMapArrayWithContext is like ApplyTWithContext, but returns a MapArrayOutput.
func (o *OutputState) ApplyMapArrayWithContext(ctx context.Context, applier interface{}) MapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(MapArrayOutput)
}

// ApplyMapMap is like ApplyT, but returns a MapMapOutput.
func (o *OutputState) ApplyMapMap(applier interface{}) MapMapOutput {
	return o.ApplyT(applier).(MapMapOutput)
}

// ApplyMapMapWithContext is like ApplyTWithContext, but returns a MapMapOutput.
func (o *OutputState) ApplyMapMapWithContext(ctx context.Context, applier interface{}) MapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(MapMapOutput)
}

// ApplyArrayArray is like ApplyT, but returns a ArrayArrayOutput.
func (o *OutputState) ApplyArrayArray(applier interface{}) ArrayArrayOutput {
	return o.ApplyT(applier).(ArrayArrayOutput)
}

// ApplyArrayArrayWithContext is like ApplyTWithContext, but returns a ArrayArrayOutput.
func (o *OutputState) ApplyArrayArrayWithContext(ctx context.Context, applier interface{}) ArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(ArrayArrayOutput)
}

// ApplyInt is like ApplyT, but returns a IntOutput.
func (o *OutputState) ApplyInt(applier interface{}) IntOutput {
	return o.ApplyT(applier).(IntOutput)
}

// ApplyIntWithContext is like ApplyTWithContext, but returns a IntOutput.
func (o *OutputState) ApplyIntWithContext(ctx context.Context, applier interface{}) IntOutput {
	return o.ApplyTWithContext(ctx, applier).(IntOutput)
}

// ApplyIntPtr is like ApplyT, but returns a IntPtrOutput.
func (o *OutputState) ApplyIntPtr(applier interface{}) IntPtrOutput {
	return o.ApplyT(applier).(IntPtrOutput)
}

// ApplyIntPtrWithContext is like ApplyTWithContext, but returns a IntPtrOutput.
func (o *OutputState) ApplyIntPtrWithContext(ctx context.Context, applier interface{}) IntPtrOutput {
	return o.ApplyTWithContext(ctx, applier).(IntPtrOutput)
}

// ApplyIntArray is like ApplyT, but returns a IntArrayOutput.
func (o *OutputState) ApplyIntArray(applier interface{}) IntArrayOutput {
	return o.ApplyT(applier).(IntArrayOutput)
}

// ApplyIntArrayWithContext is like ApplyTWithContext, but returns a IntArrayOutput.
func (o *OutputState) ApplyIntArrayWithContext(ctx context.Context, applier interface{}) IntArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IntArrayOutput)
}

// ApplyIntMap is like ApplyT, but returns a IntMapOutput.
func (o *OutputState) ApplyIntMap(applier interface{}) IntMapOutput {
	return o.ApplyT(applier).(IntMapOutput)
}

// ApplyIntMapWithContext is like ApplyTWithContext, but returns a IntMapOutput.
func (o *OutputState) ApplyIntMapWithContext(ctx context.Context, applier interface{}) IntMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IntMapOutput)
}

// ApplyIntArrayMap is like ApplyT, but returns a IntArrayMapOutput.
func (o *OutputState) ApplyIntArrayMap(applier interface{}) IntArrayMapOutput {
	return o.ApplyT(applier).(IntArrayMapOutput)
}

// ApplyIntArrayMapWithContext is like ApplyTWithContext, but returns a IntArrayMapOutput.
func (o *OutputState) ApplyIntArrayMapWithContext(ctx context.Context, applier interface{}) IntArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IntArrayMapOutput)
}

// ApplyIntMapArray is like ApplyT, but returns a IntMapArrayOutput.
func (o *OutputState) ApplyIntMapArray(applier interface{}) IntMapArrayOutput {
	return o.ApplyT(applier).(IntMapArrayOutput)
}

// ApplyIntMapArrayWithContext is like ApplyTWithContext, but returns a IntMapArrayOutput.
func (o *OutputState) ApplyIntMapArrayWithContext(ctx context.Context, applier interface{}) IntMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IntMapArrayOutput)
}

// ApplyIntMapMap is like ApplyT, but returns a IntMapMapOutput.
func (o *OutputState) ApplyIntMapMap(applier interface{}) IntMapMapOutput {
	return o.ApplyT(applier).(IntMapMapOutput)
}

// ApplyIntMapMapWithContext is like ApplyTWithContext, but returns a IntMapMapOutput.
func (o *OutputState) ApplyIntMapMapWithContext(ctx context.Context, applier interface{}) IntMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(IntMapMapOutput)
}

// ApplyIntArrayArray is like ApplyT, but returns a IntArrayArrayOutput.
func (o *OutputState) ApplyIntArrayArray(applier interface{}) IntArrayArrayOutput {
	return o.ApplyT(applier).(IntArrayArrayOutput)
}

// ApplyIntArrayArrayWithContext is like ApplyTWithContext, but returns a IntArrayArrayOutput.
func (o *OutputState) ApplyIntArrayArrayWithContext(ctx context.Context, applier interface{}) IntArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(IntArrayArrayOutput)
}

// ApplyString is like ApplyT, but returns a StringOutput.
func (o *OutputState) ApplyString(applier interface{}) StringOutput {
	return o.ApplyT(applier).(StringOutput)
}

// ApplyStringWithContext is like ApplyTWithContext, but returns a StringOutput.
func (o *OutputState) ApplyStringWithContext(ctx context.Context, applier interface{}) StringOutput {
	return o.ApplyTWithContext(ctx, applier).(StringOutput)
}

// ApplyStringPtr is like ApplyT, but returns a StringPtrOutput.
func (o *OutputState) ApplyStringPtr(applier interface{}) StringPtrOutput {
	return o.ApplyT(applier).(StringPtrOutput)
}

// ApplyStringPtrWithContext is like ApplyTWithContext, but returns a StringPtrOutput.
func (o *OutputState) ApplyStringPtrWithContext(ctx context.Context, applier interface{}) StringPtrOutput {
	return o.ApplyTWithContext(ctx, applier).(StringPtrOutput)
}

// ApplyStringArray is like ApplyT, but returns a StringArrayOutput.
func (o *OutputState) ApplyStringArray(applier interface{}) StringArrayOutput {
	return o.ApplyT(applier).(StringArrayOutput)
}

// ApplyStringArrayWithContext is like ApplyTWithContext, but returns a StringArrayOutput.
func (o *OutputState) ApplyStringArrayWithContext(ctx context.Context, applier interface{}) StringArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(StringArrayOutput)
}

// ApplyStringMap is like ApplyT, but returns a StringMapOutput.
func (o *OutputState) ApplyStringMap(applier interface{}) StringMapOutput {
	return o.ApplyT(applier).(StringMapOutput)
}

// ApplyStringMapWithContext is like ApplyTWithContext, but returns a StringMapOutput.
func (o *OutputState) ApplyStringMapWithContext(ctx context.Context, applier interface{}) StringMapOutput {
	return o.ApplyTWithContext(ctx, applier).(StringMapOutput)
}

// ApplyStringArrayMap is like ApplyT, but returns a StringArrayMapOutput.
func (o *OutputState) ApplyStringArrayMap(applier interface{}) StringArrayMapOutput {
	return o.ApplyT(applier).(StringArrayMapOutput)
}

// ApplyStringArrayMapWithContext is like ApplyTWithContext, but returns a StringArrayMapOutput.
func (o *OutputState) ApplyStringArrayMapWithContext(ctx context.Context, applier interface{}) StringArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(StringArrayMapOutput)
}

// ApplyStringMapArray is like ApplyT, but returns a StringMapArrayOutput.
func (o *OutputState) ApplyStringMapArray(applier interface{}) StringMapArrayOutput {
	return o.ApplyT(applier).(StringMapArrayOutput)
}

// ApplyStringMapArrayWithContext is like ApplyTWithContext, but returns a StringMapArrayOutput.
func (o *OutputState) ApplyStringMapArrayWithContext(ctx context.Context, applier interface{}) StringMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(StringMapArrayOutput)
}

// ApplyStringMapMap is like ApplyT, but returns a StringMapMapOutput.
func (o *OutputState) ApplyStringMapMap(applier interface{}) StringMapMapOutput {
	return o.ApplyT(applier).(StringMapMapOutput)
}

// ApplyStringMapMapWithContext is like ApplyTWithContext, but returns a StringMapMapOutput.
func (o *OutputState) ApplyStringMapMapWithContext(ctx context.Context, applier interface{}) StringMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(StringMapMapOutput)
}

// ApplyStringArrayArray is like ApplyT, but returns a StringArrayArrayOutput.
func (o *OutputState) ApplyStringArrayArray(applier interface{}) StringArrayArrayOutput {
	return o.ApplyT(applier).(StringArrayArrayOutput)
}

// ApplyStringArrayArrayWithContext is like ApplyTWithContext, but returns a StringArrayArrayOutput.
func (o *OutputState) ApplyStringArrayArrayWithContext(ctx context.Context, applier interface{}) StringArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(StringArrayArrayOutput)
}

// ApplyURN is like ApplyT, but returns a URNOutput.
func (o *OutputState) ApplyURN(applier interface{}) URNOutput {
	return o.ApplyT(applier).(URNOutput)
}

// ApplyURNWithContext is like ApplyTWithContext, but returns a URNOutput.
func (o *OutputState) ApplyURNWithContext(ctx context.Context, applier interface{}) URNOutput {
	return o.ApplyTWithContext(ctx, applier).(URNOutput)
}

// ApplyURNPtr is like ApplyT, but returns a URNPtrOutput.
func (o *OutputState) ApplyURNPtr(applier interface{}) URNPtrOutput {
	return o.ApplyT(applier).(URNPtrOutput)
}

// ApplyURNPtrWithContext is like ApplyTWithContext, but returns a URNPtrOutput.
func (o *OutputState) ApplyURNPtrWithContext(ctx context.Context, applier interface{}) URNPtrOutput {
	return o.ApplyTWithContext(ctx, applier).(URNPtrOutput)
}

// ApplyURNArray is like ApplyT, but returns a URNArrayOutput.
func (o *OutputState) ApplyURNArray(applier interface{}) URNArrayOutput {
	return o.ApplyT(applier).(URNArrayOutput)
}

// ApplyURNArrayWithContext is like ApplyTWithContext, but returns a URNArrayOutput.
func (o *OutputState) ApplyURNArrayWithContext(ctx context.Context, applier interface{}) URNArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(URNArrayOutput)
}

// ApplyURNMap is like ApplyT, but returns a URNMapOutput.
func (o *OutputState) ApplyURNMap(applier interface{}) URNMapOutput {
	return o.ApplyT(applier).(URNMapOutput)
}

// ApplyURNMapWithContext is like ApplyTWithContext, but returns a URNMapOutput.
func (o *OutputState) ApplyURNMapWithContext(ctx context.Context, applier interface{}) URNMapOutput {
	return o.ApplyTWithContext(ctx, applier).(URNMapOutput)
}

// ApplyURNArrayMap is like ApplyT, but returns a URNArrayMapOutput.
func (o *OutputState) ApplyURNArrayMap(applier interface{}) URNArrayMapOutput {
	return o.ApplyT(applier).(URNArrayMapOutput)
}

// ApplyURNArrayMapWithContext is like ApplyTWithContext, but returns a URNArrayMapOutput.
func (o *OutputState) ApplyURNArrayMapWithContext(ctx context.Context, applier interface{}) URNArrayMapOutput {
	return o.ApplyTWithContext(ctx, applier).(URNArrayMapOutput)
}

// ApplyURNMapArray is like ApplyT, but returns a URNMapArrayOutput.
func (o *OutputState) ApplyURNMapArray(applier interface{}) URNMapArrayOutput {
	return o.ApplyT(applier).(URNMapArrayOutput)
}

// ApplyURNMapArrayWithContext is like ApplyTWithContext, but returns a URNMapArrayOutput.
func (o *OutputState) ApplyURNMapArrayWithContext(ctx context.Context, applier interface{}) URNMapArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(URNMapArrayOutput)
}

// ApplyURNMapMap is like ApplyT, but returns a URNMapMapOutput.
func (o *OutputState) ApplyURNMapMap(applier interface{}) URNMapMapOutput {
	return o.ApplyT(applier).(URNMapMapOutput)
}

// ApplyURNMapMapWithContext is like ApplyTWithContext, but returns a URNMapMapOutput.
func (o *OutputState) ApplyURNMapMapWithContext(ctx context.Context, applier interface{}) URNMapMapOutput {
	return o.ApplyTWithContext(ctx, applier).(URNMapMapOutput)
}

// ApplyURNArrayArray is like ApplyT, but returns a URNArrayArrayOutput.
func (o *OutputState) ApplyURNArrayArray(applier interface{}) URNArrayArrayOutput {
	return o.ApplyT(applier).(URNArrayArrayOutput)
}

// ApplyURNArrayArrayWithContext is like ApplyTWithContext, but returns a URNArrayArrayOutput.
func (o *OutputState) ApplyURNArrayArrayWithContext(ctx context.Context, applier interface{}) URNArrayArrayOutput {
	return o.ApplyTWithContext(ctx, applier).(URNArrayArrayOutput)
}

var archiveType = reflect.TypeOf((*Archive)(nil)).Elem()

// ArchiveInput is an input type that accepts Archive and ArchiveOutput values.
type ArchiveInput interface {
	Input

	ToArchiveOutput() ArchiveOutput
	ToArchiveOutputWithContext(ctx context.Context) ArchiveOutput
}

// ElementType returns the element type of this Input (Archive).
func (*archive) ElementType() reflect.Type {
	return archiveType
}

func (in *archive) ToArchiveOutput() ArchiveOutput {
	return ToOutput(in).(ArchiveOutput)
}

func (in *archive) ToArchiveOutputWithContext(ctx context.Context) ArchiveOutput {
	return ToOutputWithContext(ctx, in).(ArchiveOutput)
}

func (in *archive) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return in.ToAssetOrArchiveOutputWithContext(context.Background())
}

func (in *archive) ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput {
	return in.ToArchiveOutputWithContext(ctx).ToAssetOrArchiveOutputWithContext(ctx)
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

func (o ArchiveOutput) ToArchiveOutputWithContext(ctx context.Context) ArchiveOutput {
	return o
}

func (o ArchiveOutput) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return o.ToAssetOrArchiveOutputWithContext(context.Background())
}

func (o ArchiveOutput) ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Archive) AssetOrArchive {
		return (AssetOrArchive)(v)
	}).(AssetOrArchiveOutput)
}

var archiveArrayType = reflect.TypeOf((*[]Archive)(nil)).Elem()

// ArchiveArrayInput is an input type that accepts ArchiveArray and ArchiveArrayOutput values.
type ArchiveArrayInput interface {
	Input

	ToArchiveArrayOutput() ArchiveArrayOutput
	ToArchiveArrayOutputWithContext(ctx context.Context) ArchiveArrayOutput
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

func (in ArchiveArray) ToArchiveArrayOutputWithContext(ctx context.Context) ArchiveArrayOutput {
	return ToOutputWithContext(ctx, in).(ArchiveArrayOutput)
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

func (o ArchiveArrayOutput) ToArchiveArrayOutputWithContext(ctx context.Context) ArchiveArrayOutput {
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
	ToArchiveMapOutputWithContext(ctx context.Context) ArchiveMapOutput
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

func (in ArchiveMap) ToArchiveMapOutputWithContext(ctx context.Context) ArchiveMapOutput {
	return ToOutputWithContext(ctx, in).(ArchiveMapOutput)
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

func (o ArchiveMapOutput) ToArchiveMapOutputWithContext(ctx context.Context) ArchiveMapOutput {
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
	ToArchiveArrayMapOutputWithContext(ctx context.Context) ArchiveArrayMapOutput
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

func (in ArchiveArrayMap) ToArchiveArrayMapOutputWithContext(ctx context.Context) ArchiveArrayMapOutput {
	return ToOutputWithContext(ctx, in).(ArchiveArrayMapOutput)
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

func (o ArchiveArrayMapOutput) ToArchiveArrayMapOutputWithContext(ctx context.Context) ArchiveArrayMapOutput {
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
	ToArchiveMapArrayOutputWithContext(ctx context.Context) ArchiveMapArrayOutput
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

func (in ArchiveMapArray) ToArchiveMapArrayOutputWithContext(ctx context.Context) ArchiveMapArrayOutput {
	return ToOutputWithContext(ctx, in).(ArchiveMapArrayOutput)
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

func (o ArchiveMapArrayOutput) ToArchiveMapArrayOutputWithContext(ctx context.Context) ArchiveMapArrayOutput {
	return o
}

func (o ArchiveMapArrayOutput) Index(i IntInput) ArchiveMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]Archive {
		return vs[0].([]map[string]Archive)[vs[1].(int)]
	}).(ArchiveMapOutput)
}

var archiveMapMapType = reflect.TypeOf((*map[string]map[string]Archive)(nil)).Elem()

// ArchiveMapMapInput is an input type that accepts ArchiveMapMap and ArchiveMapMapOutput values.
type ArchiveMapMapInput interface {
	Input

	ToArchiveMapMapOutput() ArchiveMapMapOutput
	ToArchiveMapMapOutputWithContext(ctx context.Context) ArchiveMapMapOutput
}

// ArchiveMapMap is an input type for map[string]ArchiveMapInput values.
type ArchiveMapMap map[string]ArchiveMapInput

// ElementType returns the element type of this Input (map[string]map[string]Archive).
func (ArchiveMapMap) ElementType() reflect.Type {
	return archiveMapMapType
}

func (in ArchiveMapMap) ToArchiveMapMapOutput() ArchiveMapMapOutput {
	return ToOutput(in).(ArchiveMapMapOutput)
}

func (in ArchiveMapMap) ToArchiveMapMapOutputWithContext(ctx context.Context) ArchiveMapMapOutput {
	return ToOutputWithContext(ctx, in).(ArchiveMapMapOutput)
}

// ArchiveMapMapOutput is an Output that returns map[string]map[string]Archive values.
type ArchiveMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]Archive).
func (ArchiveMapMapOutput) ElementType() reflect.Type {
	return archiveMapMapType
}

func (o ArchiveMapMapOutput) ToArchiveMapMapOutput() ArchiveMapMapOutput {
	return o
}

func (o ArchiveMapMapOutput) ToArchiveMapMapOutputWithContext(ctx context.Context) ArchiveMapMapOutput {
	return o
}

func (o ArchiveMapMapOutput) MapIndex(k StringInput) ArchiveMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]Archive {
		return vs[0].(map[string]map[string]Archive)[vs[1].(string)]
	}).(ArchiveMapOutput)
}

var archiveArrayArrayType = reflect.TypeOf((*[][]Archive)(nil)).Elem()

// ArchiveArrayArrayInput is an input type that accepts ArchiveArrayArray and ArchiveArrayArrayOutput values.
type ArchiveArrayArrayInput interface {
	Input

	ToArchiveArrayArrayOutput() ArchiveArrayArrayOutput
	ToArchiveArrayArrayOutputWithContext(ctx context.Context) ArchiveArrayArrayOutput
}

// ArchiveArrayArray is an input type for []ArchiveArrayInput values.
type ArchiveArrayArray []ArchiveArrayInput

// ElementType returns the element type of this Input ([][]Archive).
func (ArchiveArrayArray) ElementType() reflect.Type {
	return archiveArrayArrayType
}

func (in ArchiveArrayArray) ToArchiveArrayArrayOutput() ArchiveArrayArrayOutput {
	return ToOutput(in).(ArchiveArrayArrayOutput)
}

func (in ArchiveArrayArray) ToArchiveArrayArrayOutputWithContext(ctx context.Context) ArchiveArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(ArchiveArrayArrayOutput)
}

// ArchiveArrayArrayOutput is an Output that returns [][]Archive values.
type ArchiveArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]Archive).
func (ArchiveArrayArrayOutput) ElementType() reflect.Type {
	return archiveArrayArrayType
}

func (o ArchiveArrayArrayOutput) ToArchiveArrayArrayOutput() ArchiveArrayArrayOutput {
	return o
}

func (o ArchiveArrayArrayOutput) ToArchiveArrayArrayOutputWithContext(ctx context.Context) ArchiveArrayArrayOutput {
	return o
}

func (o ArchiveArrayArrayOutput) Index(i IntInput) ArchiveArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []Archive {
		return vs[0].([][]Archive)[vs[1].(int)]
	}).(ArchiveArrayOutput)
}

var assetType = reflect.TypeOf((*Asset)(nil)).Elem()

// AssetInput is an input type that accepts Asset and AssetOutput values.
type AssetInput interface {
	Input

	ToAssetOutput() AssetOutput
	ToAssetOutputWithContext(ctx context.Context) AssetOutput
}

// ElementType returns the element type of this Input (Asset).
func (*asset) ElementType() reflect.Type {
	return assetType
}

func (in *asset) ToAssetOutput() AssetOutput {
	return ToOutput(in).(AssetOutput)
}

func (in *asset) ToAssetOutputWithContext(ctx context.Context) AssetOutput {
	return ToOutputWithContext(ctx, in).(AssetOutput)
}

func (in *asset) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return in.ToAssetOrArchiveOutputWithContext(context.Background())
}

func (in *asset) ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput {
	return in.ToAssetOutputWithContext(ctx).ToAssetOrArchiveOutputWithContext(ctx)
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

func (o AssetOutput) ToAssetOutputWithContext(ctx context.Context) AssetOutput {
	return o
}

func (o AssetOutput) ToAssetOrArchiveOutput() AssetOrArchiveOutput {
	return o.ToAssetOrArchiveOutputWithContext(context.Background())
}

func (o AssetOutput) ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v Asset) AssetOrArchive {
		return (AssetOrArchive)(v)
	}).(AssetOrArchiveOutput)
}

var assetArrayType = reflect.TypeOf((*[]Asset)(nil)).Elem()

// AssetArrayInput is an input type that accepts AssetArray and AssetArrayOutput values.
type AssetArrayInput interface {
	Input

	ToAssetArrayOutput() AssetArrayOutput
	ToAssetArrayOutputWithContext(ctx context.Context) AssetArrayOutput
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

func (in AssetArray) ToAssetArrayOutputWithContext(ctx context.Context) AssetArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetArrayOutput)
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

func (o AssetArrayOutput) ToAssetArrayOutputWithContext(ctx context.Context) AssetArrayOutput {
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
	ToAssetMapOutputWithContext(ctx context.Context) AssetMapOutput
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

func (in AssetMap) ToAssetMapOutputWithContext(ctx context.Context) AssetMapOutput {
	return ToOutputWithContext(ctx, in).(AssetMapOutput)
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

func (o AssetMapOutput) ToAssetMapOutputWithContext(ctx context.Context) AssetMapOutput {
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
	ToAssetArrayMapOutputWithContext(ctx context.Context) AssetArrayMapOutput
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

func (in AssetArrayMap) ToAssetArrayMapOutputWithContext(ctx context.Context) AssetArrayMapOutput {
	return ToOutputWithContext(ctx, in).(AssetArrayMapOutput)
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

func (o AssetArrayMapOutput) ToAssetArrayMapOutputWithContext(ctx context.Context) AssetArrayMapOutput {
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
	ToAssetMapArrayOutputWithContext(ctx context.Context) AssetMapArrayOutput
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

func (in AssetMapArray) ToAssetMapArrayOutputWithContext(ctx context.Context) AssetMapArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetMapArrayOutput)
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

func (o AssetMapArrayOutput) ToAssetMapArrayOutputWithContext(ctx context.Context) AssetMapArrayOutput {
	return o
}

func (o AssetMapArrayOutput) Index(i IntInput) AssetMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]Asset {
		return vs[0].([]map[string]Asset)[vs[1].(int)]
	}).(AssetMapOutput)
}

var assetMapMapType = reflect.TypeOf((*map[string]map[string]Asset)(nil)).Elem()

// AssetMapMapInput is an input type that accepts AssetMapMap and AssetMapMapOutput values.
type AssetMapMapInput interface {
	Input

	ToAssetMapMapOutput() AssetMapMapOutput
	ToAssetMapMapOutputWithContext(ctx context.Context) AssetMapMapOutput
}

// AssetMapMap is an input type for map[string]AssetMapInput values.
type AssetMapMap map[string]AssetMapInput

// ElementType returns the element type of this Input (map[string]map[string]Asset).
func (AssetMapMap) ElementType() reflect.Type {
	return assetMapMapType
}

func (in AssetMapMap) ToAssetMapMapOutput() AssetMapMapOutput {
	return ToOutput(in).(AssetMapMapOutput)
}

func (in AssetMapMap) ToAssetMapMapOutputWithContext(ctx context.Context) AssetMapMapOutput {
	return ToOutputWithContext(ctx, in).(AssetMapMapOutput)
}

// AssetMapMapOutput is an Output that returns map[string]map[string]Asset values.
type AssetMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]Asset).
func (AssetMapMapOutput) ElementType() reflect.Type {
	return assetMapMapType
}

func (o AssetMapMapOutput) ToAssetMapMapOutput() AssetMapMapOutput {
	return o
}

func (o AssetMapMapOutput) ToAssetMapMapOutputWithContext(ctx context.Context) AssetMapMapOutput {
	return o
}

func (o AssetMapMapOutput) MapIndex(k StringInput) AssetMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]Asset {
		return vs[0].(map[string]map[string]Asset)[vs[1].(string)]
	}).(AssetMapOutput)
}

var assetArrayArrayType = reflect.TypeOf((*[][]Asset)(nil)).Elem()

// AssetArrayArrayInput is an input type that accepts AssetArrayArray and AssetArrayArrayOutput values.
type AssetArrayArrayInput interface {
	Input

	ToAssetArrayArrayOutput() AssetArrayArrayOutput
	ToAssetArrayArrayOutputWithContext(ctx context.Context) AssetArrayArrayOutput
}

// AssetArrayArray is an input type for []AssetArrayInput values.
type AssetArrayArray []AssetArrayInput

// ElementType returns the element type of this Input ([][]Asset).
func (AssetArrayArray) ElementType() reflect.Type {
	return assetArrayArrayType
}

func (in AssetArrayArray) ToAssetArrayArrayOutput() AssetArrayArrayOutput {
	return ToOutput(in).(AssetArrayArrayOutput)
}

func (in AssetArrayArray) ToAssetArrayArrayOutputWithContext(ctx context.Context) AssetArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetArrayArrayOutput)
}

// AssetArrayArrayOutput is an Output that returns [][]Asset values.
type AssetArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]Asset).
func (AssetArrayArrayOutput) ElementType() reflect.Type {
	return assetArrayArrayType
}

func (o AssetArrayArrayOutput) ToAssetArrayArrayOutput() AssetArrayArrayOutput {
	return o
}

func (o AssetArrayArrayOutput) ToAssetArrayArrayOutputWithContext(ctx context.Context) AssetArrayArrayOutput {
	return o
}

func (o AssetArrayArrayOutput) Index(i IntInput) AssetArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []Asset {
		return vs[0].([][]Asset)[vs[1].(int)]
	}).(AssetArrayOutput)
}

var assetOrArchiveType = reflect.TypeOf((*AssetOrArchive)(nil)).Elem()

// AssetOrArchiveInput is an input type that accepts AssetOrArchive and AssetOrArchiveOutput values.
type AssetOrArchiveInput interface {
	Input

	ToAssetOrArchiveOutput() AssetOrArchiveOutput
	ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput
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

func (o AssetOrArchiveOutput) ToAssetOrArchiveOutputWithContext(ctx context.Context) AssetOrArchiveOutput {
	return o
}

var assetOrArchiveArrayType = reflect.TypeOf((*[]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveArrayInput is an input type that accepts AssetOrArchiveArray and AssetOrArchiveArrayOutput values.
type AssetOrArchiveArrayInput interface {
	Input

	ToAssetOrArchiveArrayOutput() AssetOrArchiveArrayOutput
	ToAssetOrArchiveArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayOutput
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

func (in AssetOrArchiveArray) ToAssetOrArchiveArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveArrayOutput)
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

func (o AssetOrArchiveArrayOutput) ToAssetOrArchiveArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayOutput {
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
	ToAssetOrArchiveMapOutputWithContext(ctx context.Context) AssetOrArchiveMapOutput
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

func (in AssetOrArchiveMap) ToAssetOrArchiveMapOutputWithContext(ctx context.Context) AssetOrArchiveMapOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveMapOutput)
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

func (o AssetOrArchiveMapOutput) ToAssetOrArchiveMapOutputWithContext(ctx context.Context) AssetOrArchiveMapOutput {
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
	ToAssetOrArchiveArrayMapOutputWithContext(ctx context.Context) AssetOrArchiveArrayMapOutput
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

func (in AssetOrArchiveArrayMap) ToAssetOrArchiveArrayMapOutputWithContext(ctx context.Context) AssetOrArchiveArrayMapOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveArrayMapOutput)
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

func (o AssetOrArchiveArrayMapOutput) ToAssetOrArchiveArrayMapOutputWithContext(ctx context.Context) AssetOrArchiveArrayMapOutput {
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
	ToAssetOrArchiveMapArrayOutputWithContext(ctx context.Context) AssetOrArchiveMapArrayOutput
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

func (in AssetOrArchiveMapArray) ToAssetOrArchiveMapArrayOutputWithContext(ctx context.Context) AssetOrArchiveMapArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveMapArrayOutput)
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

func (o AssetOrArchiveMapArrayOutput) ToAssetOrArchiveMapArrayOutputWithContext(ctx context.Context) AssetOrArchiveMapArrayOutput {
	return o
}

func (o AssetOrArchiveMapArrayOutput) Index(i IntInput) AssetOrArchiveMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]AssetOrArchive {
		return vs[0].([]map[string]AssetOrArchive)[vs[1].(int)]
	}).(AssetOrArchiveMapOutput)
}

var assetOrArchiveMapMapType = reflect.TypeOf((*map[string]map[string]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveMapMapInput is an input type that accepts AssetOrArchiveMapMap and AssetOrArchiveMapMapOutput values.
type AssetOrArchiveMapMapInput interface {
	Input

	ToAssetOrArchiveMapMapOutput() AssetOrArchiveMapMapOutput
	ToAssetOrArchiveMapMapOutputWithContext(ctx context.Context) AssetOrArchiveMapMapOutput
}

// AssetOrArchiveMapMap is an input type for map[string]AssetOrArchiveMapInput values.
type AssetOrArchiveMapMap map[string]AssetOrArchiveMapInput

// ElementType returns the element type of this Input (map[string]map[string]AssetOrArchive).
func (AssetOrArchiveMapMap) ElementType() reflect.Type {
	return assetOrArchiveMapMapType
}

func (in AssetOrArchiveMapMap) ToAssetOrArchiveMapMapOutput() AssetOrArchiveMapMapOutput {
	return ToOutput(in).(AssetOrArchiveMapMapOutput)
}

func (in AssetOrArchiveMapMap) ToAssetOrArchiveMapMapOutputWithContext(ctx context.Context) AssetOrArchiveMapMapOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveMapMapOutput)
}

// AssetOrArchiveMapMapOutput is an Output that returns map[string]map[string]AssetOrArchive values.
type AssetOrArchiveMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]AssetOrArchive).
func (AssetOrArchiveMapMapOutput) ElementType() reflect.Type {
	return assetOrArchiveMapMapType
}

func (o AssetOrArchiveMapMapOutput) ToAssetOrArchiveMapMapOutput() AssetOrArchiveMapMapOutput {
	return o
}

func (o AssetOrArchiveMapMapOutput) ToAssetOrArchiveMapMapOutputWithContext(ctx context.Context) AssetOrArchiveMapMapOutput {
	return o
}

func (o AssetOrArchiveMapMapOutput) MapIndex(k StringInput) AssetOrArchiveMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]AssetOrArchive {
		return vs[0].(map[string]map[string]AssetOrArchive)[vs[1].(string)]
	}).(AssetOrArchiveMapOutput)
}

var assetOrArchiveArrayArrayType = reflect.TypeOf((*[][]AssetOrArchive)(nil)).Elem()

// AssetOrArchiveArrayArrayInput is an input type that accepts AssetOrArchiveArrayArray and AssetOrArchiveArrayArrayOutput values.
type AssetOrArchiveArrayArrayInput interface {
	Input

	ToAssetOrArchiveArrayArrayOutput() AssetOrArchiveArrayArrayOutput
	ToAssetOrArchiveArrayArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayArrayOutput
}

// AssetOrArchiveArrayArray is an input type for []AssetOrArchiveArrayInput values.
type AssetOrArchiveArrayArray []AssetOrArchiveArrayInput

// ElementType returns the element type of this Input ([][]AssetOrArchive).
func (AssetOrArchiveArrayArray) ElementType() reflect.Type {
	return assetOrArchiveArrayArrayType
}

func (in AssetOrArchiveArrayArray) ToAssetOrArchiveArrayArrayOutput() AssetOrArchiveArrayArrayOutput {
	return ToOutput(in).(AssetOrArchiveArrayArrayOutput)
}

func (in AssetOrArchiveArrayArray) ToAssetOrArchiveArrayArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(AssetOrArchiveArrayArrayOutput)
}

// AssetOrArchiveArrayArrayOutput is an Output that returns [][]AssetOrArchive values.
type AssetOrArchiveArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]AssetOrArchive).
func (AssetOrArchiveArrayArrayOutput) ElementType() reflect.Type {
	return assetOrArchiveArrayArrayType
}

func (o AssetOrArchiveArrayArrayOutput) ToAssetOrArchiveArrayArrayOutput() AssetOrArchiveArrayArrayOutput {
	return o
}

func (o AssetOrArchiveArrayArrayOutput) ToAssetOrArchiveArrayArrayOutputWithContext(ctx context.Context) AssetOrArchiveArrayArrayOutput {
	return o
}

func (o AssetOrArchiveArrayArrayOutput) Index(i IntInput) AssetOrArchiveArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []AssetOrArchive {
		return vs[0].([][]AssetOrArchive)[vs[1].(int)]
	}).(AssetOrArchiveArrayOutput)
}

var boolType = reflect.TypeOf((*bool)(nil)).Elem()

// BoolInput is an input type that accepts Bool and BoolOutput values.
type BoolInput interface {
	Input

	ToBoolOutput() BoolOutput
	ToBoolOutputWithContext(ctx context.Context) BoolOutput

	ToBoolPtrOutput() BoolPtrOutput
	ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput
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

func (in Bool) ToBoolOutputWithContext(ctx context.Context) BoolOutput {
	return ToOutputWithContext(ctx, in).(BoolOutput)
}

func (in Bool) ToBoolPtrOutput() BoolPtrOutput {
	return in.ToBoolPtrOutputWithContext(context.Background())
}

func (in Bool) ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput {
	return in.ToBoolOutputWithContext(ctx).ToBoolPtrOutputWithContext(ctx)
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

func (o BoolOutput) ToBoolOutputWithContext(ctx context.Context) BoolOutput {
	return o
}

func (o BoolOutput) ToBoolPtrOutput() BoolPtrOutput {
	return o.ToBoolPtrOutputWithContext(context.Background())
}

func (o BoolOutput) ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v bool) *bool {
		return &v
	}).(BoolPtrOutput)
}

var boolPtrType = reflect.TypeOf((**bool)(nil)).Elem()

// BoolPtrInput is an input type that accepts BoolPtr and BoolPtrOutput values.
type BoolPtrInput interface {
	Input

	ToBoolPtrOutput() BoolPtrOutput
	ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput
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

func (in *boolPtr) ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput {
	return ToOutputWithContext(ctx, in).(BoolPtrOutput)
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

func (o BoolPtrOutput) ToBoolPtrOutputWithContext(ctx context.Context) BoolPtrOutput {
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
	ToBoolArrayOutputWithContext(ctx context.Context) BoolArrayOutput
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

func (in BoolArray) ToBoolArrayOutputWithContext(ctx context.Context) BoolArrayOutput {
	return ToOutputWithContext(ctx, in).(BoolArrayOutput)
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

func (o BoolArrayOutput) ToBoolArrayOutputWithContext(ctx context.Context) BoolArrayOutput {
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
	ToBoolMapOutputWithContext(ctx context.Context) BoolMapOutput
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

func (in BoolMap) ToBoolMapOutputWithContext(ctx context.Context) BoolMapOutput {
	return ToOutputWithContext(ctx, in).(BoolMapOutput)
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

func (o BoolMapOutput) ToBoolMapOutputWithContext(ctx context.Context) BoolMapOutput {
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
	ToBoolArrayMapOutputWithContext(ctx context.Context) BoolArrayMapOutput
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

func (in BoolArrayMap) ToBoolArrayMapOutputWithContext(ctx context.Context) BoolArrayMapOutput {
	return ToOutputWithContext(ctx, in).(BoolArrayMapOutput)
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

func (o BoolArrayMapOutput) ToBoolArrayMapOutputWithContext(ctx context.Context) BoolArrayMapOutput {
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
	ToBoolMapArrayOutputWithContext(ctx context.Context) BoolMapArrayOutput
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

func (in BoolMapArray) ToBoolMapArrayOutputWithContext(ctx context.Context) BoolMapArrayOutput {
	return ToOutputWithContext(ctx, in).(BoolMapArrayOutput)
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

func (o BoolMapArrayOutput) ToBoolMapArrayOutputWithContext(ctx context.Context) BoolMapArrayOutput {
	return o
}

func (o BoolMapArrayOutput) Index(i IntInput) BoolMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]bool {
		return vs[0].([]map[string]bool)[vs[1].(int)]
	}).(BoolMapOutput)
}

var boolMapMapType = reflect.TypeOf((*map[string]map[string]bool)(nil)).Elem()

// BoolMapMapInput is an input type that accepts BoolMapMap and BoolMapMapOutput values.
type BoolMapMapInput interface {
	Input

	ToBoolMapMapOutput() BoolMapMapOutput
	ToBoolMapMapOutputWithContext(ctx context.Context) BoolMapMapOutput
}

// BoolMapMap is an input type for map[string]BoolMapInput values.
type BoolMapMap map[string]BoolMapInput

// ElementType returns the element type of this Input (map[string]map[string]bool).
func (BoolMapMap) ElementType() reflect.Type {
	return boolMapMapType
}

func (in BoolMapMap) ToBoolMapMapOutput() BoolMapMapOutput {
	return ToOutput(in).(BoolMapMapOutput)
}

func (in BoolMapMap) ToBoolMapMapOutputWithContext(ctx context.Context) BoolMapMapOutput {
	return ToOutputWithContext(ctx, in).(BoolMapMapOutput)
}

// BoolMapMapOutput is an Output that returns map[string]map[string]bool values.
type BoolMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]bool).
func (BoolMapMapOutput) ElementType() reflect.Type {
	return boolMapMapType
}

func (o BoolMapMapOutput) ToBoolMapMapOutput() BoolMapMapOutput {
	return o
}

func (o BoolMapMapOutput) ToBoolMapMapOutputWithContext(ctx context.Context) BoolMapMapOutput {
	return o
}

func (o BoolMapMapOutput) MapIndex(k StringInput) BoolMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]bool {
		return vs[0].(map[string]map[string]bool)[vs[1].(string)]
	}).(BoolMapOutput)
}

var boolArrayArrayType = reflect.TypeOf((*[][]bool)(nil)).Elem()

// BoolArrayArrayInput is an input type that accepts BoolArrayArray and BoolArrayArrayOutput values.
type BoolArrayArrayInput interface {
	Input

	ToBoolArrayArrayOutput() BoolArrayArrayOutput
	ToBoolArrayArrayOutputWithContext(ctx context.Context) BoolArrayArrayOutput
}

// BoolArrayArray is an input type for []BoolArrayInput values.
type BoolArrayArray []BoolArrayInput

// ElementType returns the element type of this Input ([][]bool).
func (BoolArrayArray) ElementType() reflect.Type {
	return boolArrayArrayType
}

func (in BoolArrayArray) ToBoolArrayArrayOutput() BoolArrayArrayOutput {
	return ToOutput(in).(BoolArrayArrayOutput)
}

func (in BoolArrayArray) ToBoolArrayArrayOutputWithContext(ctx context.Context) BoolArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(BoolArrayArrayOutput)
}

// BoolArrayArrayOutput is an Output that returns [][]bool values.
type BoolArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]bool).
func (BoolArrayArrayOutput) ElementType() reflect.Type {
	return boolArrayArrayType
}

func (o BoolArrayArrayOutput) ToBoolArrayArrayOutput() BoolArrayArrayOutput {
	return o
}

func (o BoolArrayArrayOutput) ToBoolArrayArrayOutputWithContext(ctx context.Context) BoolArrayArrayOutput {
	return o
}

func (o BoolArrayArrayOutput) Index(i IntInput) BoolArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []bool {
		return vs[0].([][]bool)[vs[1].(int)]
	}).(BoolArrayOutput)
}

var float64Type = reflect.TypeOf((*float64)(nil)).Elem()

// Float64Input is an input type that accepts Float64 and Float64Output values.
type Float64Input interface {
	Input

	ToFloat64Output() Float64Output
	ToFloat64OutputWithContext(ctx context.Context) Float64Output

	ToFloat64PtrOutput() Float64PtrOutput
	ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput
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

func (in Float64) ToFloat64OutputWithContext(ctx context.Context) Float64Output {
	return ToOutputWithContext(ctx, in).(Float64Output)
}

func (in Float64) ToFloat64PtrOutput() Float64PtrOutput {
	return in.ToFloat64PtrOutputWithContext(context.Background())
}

func (in Float64) ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput {
	return in.ToFloat64OutputWithContext(ctx).ToFloat64PtrOutputWithContext(ctx)
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

func (o Float64Output) ToFloat64OutputWithContext(ctx context.Context) Float64Output {
	return o
}

func (o Float64Output) ToFloat64PtrOutput() Float64PtrOutput {
	return o.ToFloat64PtrOutputWithContext(context.Background())
}

func (o Float64Output) ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v float64) *float64 {
		return &v
	}).(Float64PtrOutput)
}

var float64PtrType = reflect.TypeOf((**float64)(nil)).Elem()

// Float64PtrInput is an input type that accepts Float64Ptr and Float64PtrOutput values.
type Float64PtrInput interface {
	Input

	ToFloat64PtrOutput() Float64PtrOutput
	ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput
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

func (in *float64Ptr) ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput {
	return ToOutputWithContext(ctx, in).(Float64PtrOutput)
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

func (o Float64PtrOutput) ToFloat64PtrOutputWithContext(ctx context.Context) Float64PtrOutput {
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
	ToFloat64ArrayOutputWithContext(ctx context.Context) Float64ArrayOutput
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

func (in Float64Array) ToFloat64ArrayOutputWithContext(ctx context.Context) Float64ArrayOutput {
	return ToOutputWithContext(ctx, in).(Float64ArrayOutput)
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

func (o Float64ArrayOutput) ToFloat64ArrayOutputWithContext(ctx context.Context) Float64ArrayOutput {
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
	ToFloat64MapOutputWithContext(ctx context.Context) Float64MapOutput
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

func (in Float64Map) ToFloat64MapOutputWithContext(ctx context.Context) Float64MapOutput {
	return ToOutputWithContext(ctx, in).(Float64MapOutput)
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

func (o Float64MapOutput) ToFloat64MapOutputWithContext(ctx context.Context) Float64MapOutput {
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
	ToFloat64ArrayMapOutputWithContext(ctx context.Context) Float64ArrayMapOutput
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

func (in Float64ArrayMap) ToFloat64ArrayMapOutputWithContext(ctx context.Context) Float64ArrayMapOutput {
	return ToOutputWithContext(ctx, in).(Float64ArrayMapOutput)
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

func (o Float64ArrayMapOutput) ToFloat64ArrayMapOutputWithContext(ctx context.Context) Float64ArrayMapOutput {
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
	ToFloat64MapArrayOutputWithContext(ctx context.Context) Float64MapArrayOutput
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

func (in Float64MapArray) ToFloat64MapArrayOutputWithContext(ctx context.Context) Float64MapArrayOutput {
	return ToOutputWithContext(ctx, in).(Float64MapArrayOutput)
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

func (o Float64MapArrayOutput) ToFloat64MapArrayOutputWithContext(ctx context.Context) Float64MapArrayOutput {
	return o
}

func (o Float64MapArrayOutput) Index(i IntInput) Float64MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]float64 {
		return vs[0].([]map[string]float64)[vs[1].(int)]
	}).(Float64MapOutput)
}

var float64MapMapType = reflect.TypeOf((*map[string]map[string]float64)(nil)).Elem()

// Float64MapMapInput is an input type that accepts Float64MapMap and Float64MapMapOutput values.
type Float64MapMapInput interface {
	Input

	ToFloat64MapMapOutput() Float64MapMapOutput
	ToFloat64MapMapOutputWithContext(ctx context.Context) Float64MapMapOutput
}

// Float64MapMap is an input type for map[string]Float64MapInput values.
type Float64MapMap map[string]Float64MapInput

// ElementType returns the element type of this Input (map[string]map[string]float64).
func (Float64MapMap) ElementType() reflect.Type {
	return float64MapMapType
}

func (in Float64MapMap) ToFloat64MapMapOutput() Float64MapMapOutput {
	return ToOutput(in).(Float64MapMapOutput)
}

func (in Float64MapMap) ToFloat64MapMapOutputWithContext(ctx context.Context) Float64MapMapOutput {
	return ToOutputWithContext(ctx, in).(Float64MapMapOutput)
}

// Float64MapMapOutput is an Output that returns map[string]map[string]float64 values.
type Float64MapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]float64).
func (Float64MapMapOutput) ElementType() reflect.Type {
	return float64MapMapType
}

func (o Float64MapMapOutput) ToFloat64MapMapOutput() Float64MapMapOutput {
	return o
}

func (o Float64MapMapOutput) ToFloat64MapMapOutputWithContext(ctx context.Context) Float64MapMapOutput {
	return o
}

func (o Float64MapMapOutput) MapIndex(k StringInput) Float64MapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]float64 {
		return vs[0].(map[string]map[string]float64)[vs[1].(string)]
	}).(Float64MapOutput)
}

var float64ArrayArrayType = reflect.TypeOf((*[][]float64)(nil)).Elem()

// Float64ArrayArrayInput is an input type that accepts Float64ArrayArray and Float64ArrayArrayOutput values.
type Float64ArrayArrayInput interface {
	Input

	ToFloat64ArrayArrayOutput() Float64ArrayArrayOutput
	ToFloat64ArrayArrayOutputWithContext(ctx context.Context) Float64ArrayArrayOutput
}

// Float64ArrayArray is an input type for []Float64ArrayInput values.
type Float64ArrayArray []Float64ArrayInput

// ElementType returns the element type of this Input ([][]float64).
func (Float64ArrayArray) ElementType() reflect.Type {
	return float64ArrayArrayType
}

func (in Float64ArrayArray) ToFloat64ArrayArrayOutput() Float64ArrayArrayOutput {
	return ToOutput(in).(Float64ArrayArrayOutput)
}

func (in Float64ArrayArray) ToFloat64ArrayArrayOutputWithContext(ctx context.Context) Float64ArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(Float64ArrayArrayOutput)
}

// Float64ArrayArrayOutput is an Output that returns [][]float64 values.
type Float64ArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]float64).
func (Float64ArrayArrayOutput) ElementType() reflect.Type {
	return float64ArrayArrayType
}

func (o Float64ArrayArrayOutput) ToFloat64ArrayArrayOutput() Float64ArrayArrayOutput {
	return o
}

func (o Float64ArrayArrayOutput) ToFloat64ArrayArrayOutputWithContext(ctx context.Context) Float64ArrayArrayOutput {
	return o
}

func (o Float64ArrayArrayOutput) Index(i IntInput) Float64ArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []float64 {
		return vs[0].([][]float64)[vs[1].(int)]
	}).(Float64ArrayOutput)
}

var idType = reflect.TypeOf((*ID)(nil)).Elem()

// IDInput is an input type that accepts ID and IDOutput values.
type IDInput interface {
	Input

	ToIDOutput() IDOutput
	ToIDOutputWithContext(ctx context.Context) IDOutput

	ToIDPtrOutput() IDPtrOutput
	ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput
}

// ElementType returns the element type of this Input (ID).
func (ID) ElementType() reflect.Type {
	return idType
}

func (in ID) ToIDOutput() IDOutput {
	return ToOutput(in).(IDOutput)
}

func (in ID) ToIDOutputWithContext(ctx context.Context) IDOutput {
	return ToOutputWithContext(ctx, in).(IDOutput)
}

func (in ID) ToStringOutput() StringOutput {
	return in.ToStringOutputWithContext(context.Background())
}

func (in ID) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return in.ToIDOutputWithContext(ctx).ToStringOutputWithContext(ctx)
}

func (in ID) ToIDPtrOutput() IDPtrOutput {
	return in.ToIDPtrOutputWithContext(context.Background())
}

func (in ID) ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput {
	return in.ToIDOutputWithContext(ctx).ToIDPtrOutputWithContext(ctx)
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

func (o IDOutput) ToIDOutputWithContext(ctx context.Context) IDOutput {
	return o
}

func (o IDOutput) ToStringOutput() StringOutput {
	return o.ToStringOutputWithContext(context.Background())
}

func (o IDOutput) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v ID) string {
		return (string)(v)
	}).(StringOutput)
}

func (o IDOutput) ToIDPtrOutput() IDPtrOutput {
	return o.ToIDPtrOutputWithContext(context.Background())
}

func (o IDOutput) ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v ID) *ID {
		return &v
	}).(IDPtrOutput)
}

var iDPtrType = reflect.TypeOf((**ID)(nil)).Elem()

// IDPtrInput is an input type that accepts IDPtr and IDPtrOutput values.
type IDPtrInput interface {
	Input

	ToIDPtrOutput() IDPtrOutput
	ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput
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

func (in *idPtr) ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput {
	return ToOutputWithContext(ctx, in).(IDPtrOutput)
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

func (o IDPtrOutput) ToIDPtrOutputWithContext(ctx context.Context) IDPtrOutput {
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
	ToIDArrayOutputWithContext(ctx context.Context) IDArrayOutput
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

func (in IDArray) ToIDArrayOutputWithContext(ctx context.Context) IDArrayOutput {
	return ToOutputWithContext(ctx, in).(IDArrayOutput)
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

func (o IDArrayOutput) ToIDArrayOutputWithContext(ctx context.Context) IDArrayOutput {
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
	ToIDMapOutputWithContext(ctx context.Context) IDMapOutput
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

func (in IDMap) ToIDMapOutputWithContext(ctx context.Context) IDMapOutput {
	return ToOutputWithContext(ctx, in).(IDMapOutput)
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

func (o IDMapOutput) ToIDMapOutputWithContext(ctx context.Context) IDMapOutput {
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
	ToIDArrayMapOutputWithContext(ctx context.Context) IDArrayMapOutput
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

func (in IDArrayMap) ToIDArrayMapOutputWithContext(ctx context.Context) IDArrayMapOutput {
	return ToOutputWithContext(ctx, in).(IDArrayMapOutput)
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

func (o IDArrayMapOutput) ToIDArrayMapOutputWithContext(ctx context.Context) IDArrayMapOutput {
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
	ToIDMapArrayOutputWithContext(ctx context.Context) IDMapArrayOutput
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

func (in IDMapArray) ToIDMapArrayOutputWithContext(ctx context.Context) IDMapArrayOutput {
	return ToOutputWithContext(ctx, in).(IDMapArrayOutput)
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

func (o IDMapArrayOutput) ToIDMapArrayOutputWithContext(ctx context.Context) IDMapArrayOutput {
	return o
}

func (o IDMapArrayOutput) Index(i IntInput) IDMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]ID {
		return vs[0].([]map[string]ID)[vs[1].(int)]
	}).(IDMapOutput)
}

var iDMapMapType = reflect.TypeOf((*map[string]map[string]ID)(nil)).Elem()

// IDMapMapInput is an input type that accepts IDMapMap and IDMapMapOutput values.
type IDMapMapInput interface {
	Input

	ToIDMapMapOutput() IDMapMapOutput
	ToIDMapMapOutputWithContext(ctx context.Context) IDMapMapOutput
}

// IDMapMap is an input type for map[string]IDMapInput values.
type IDMapMap map[string]IDMapInput

// ElementType returns the element type of this Input (map[string]map[string]ID).
func (IDMapMap) ElementType() reflect.Type {
	return iDMapMapType
}

func (in IDMapMap) ToIDMapMapOutput() IDMapMapOutput {
	return ToOutput(in).(IDMapMapOutput)
}

func (in IDMapMap) ToIDMapMapOutputWithContext(ctx context.Context) IDMapMapOutput {
	return ToOutputWithContext(ctx, in).(IDMapMapOutput)
}

// IDMapMapOutput is an Output that returns map[string]map[string]ID values.
type IDMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]ID).
func (IDMapMapOutput) ElementType() reflect.Type {
	return iDMapMapType
}

func (o IDMapMapOutput) ToIDMapMapOutput() IDMapMapOutput {
	return o
}

func (o IDMapMapOutput) ToIDMapMapOutputWithContext(ctx context.Context) IDMapMapOutput {
	return o
}

func (o IDMapMapOutput) MapIndex(k StringInput) IDMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]ID {
		return vs[0].(map[string]map[string]ID)[vs[1].(string)]
	}).(IDMapOutput)
}

var iDArrayArrayType = reflect.TypeOf((*[][]ID)(nil)).Elem()

// IDArrayArrayInput is an input type that accepts IDArrayArray and IDArrayArrayOutput values.
type IDArrayArrayInput interface {
	Input

	ToIDArrayArrayOutput() IDArrayArrayOutput
	ToIDArrayArrayOutputWithContext(ctx context.Context) IDArrayArrayOutput
}

// IDArrayArray is an input type for []IDArrayInput values.
type IDArrayArray []IDArrayInput

// ElementType returns the element type of this Input ([][]ID).
func (IDArrayArray) ElementType() reflect.Type {
	return iDArrayArrayType
}

func (in IDArrayArray) ToIDArrayArrayOutput() IDArrayArrayOutput {
	return ToOutput(in).(IDArrayArrayOutput)
}

func (in IDArrayArray) ToIDArrayArrayOutputWithContext(ctx context.Context) IDArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(IDArrayArrayOutput)
}

// IDArrayArrayOutput is an Output that returns [][]ID values.
type IDArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]ID).
func (IDArrayArrayOutput) ElementType() reflect.Type {
	return iDArrayArrayType
}

func (o IDArrayArrayOutput) ToIDArrayArrayOutput() IDArrayArrayOutput {
	return o
}

func (o IDArrayArrayOutput) ToIDArrayArrayOutputWithContext(ctx context.Context) IDArrayArrayOutput {
	return o
}

func (o IDArrayArrayOutput) Index(i IntInput) IDArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []ID {
		return vs[0].([][]ID)[vs[1].(int)]
	}).(IDArrayOutput)
}

var arrayType = reflect.TypeOf((*[]interface{})(nil)).Elem()

// ArrayInput is an input type that accepts Array and ArrayOutput values.
type ArrayInput interface {
	Input

	ToArrayOutput() ArrayOutput
	ToArrayOutputWithContext(ctx context.Context) ArrayOutput
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

func (in Array) ToArrayOutputWithContext(ctx context.Context) ArrayOutput {
	return ToOutputWithContext(ctx, in).(ArrayOutput)
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

func (o ArrayOutput) ToArrayOutputWithContext(ctx context.Context) ArrayOutput {
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
	ToMapOutputWithContext(ctx context.Context) MapOutput
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

func (in Map) ToMapOutputWithContext(ctx context.Context) MapOutput {
	return ToOutputWithContext(ctx, in).(MapOutput)
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

func (o MapOutput) ToMapOutputWithContext(ctx context.Context) MapOutput {
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
	ToArrayMapOutputWithContext(ctx context.Context) ArrayMapOutput
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

func (in ArrayMap) ToArrayMapOutputWithContext(ctx context.Context) ArrayMapOutput {
	return ToOutputWithContext(ctx, in).(ArrayMapOutput)
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

func (o ArrayMapOutput) ToArrayMapOutputWithContext(ctx context.Context) ArrayMapOutput {
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
	ToMapArrayOutputWithContext(ctx context.Context) MapArrayOutput
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

func (in MapArray) ToMapArrayOutputWithContext(ctx context.Context) MapArrayOutput {
	return ToOutputWithContext(ctx, in).(MapArrayOutput)
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

func (o MapArrayOutput) ToMapArrayOutputWithContext(ctx context.Context) MapArrayOutput {
	return o
}

func (o MapArrayOutput) Index(i IntInput) MapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]interface{} {
		return vs[0].([]map[string]interface{})[vs[1].(int)]
	}).(MapOutput)
}

var mapMapType = reflect.TypeOf((*map[string]map[string]interface{})(nil)).Elem()

// MapMapInput is an input type that accepts MapMap and MapMapOutput values.
type MapMapInput interface {
	Input

	ToMapMapOutput() MapMapOutput
	ToMapMapOutputWithContext(ctx context.Context) MapMapOutput
}

// MapMap is an input type for map[string]MapInput values.
type MapMap map[string]MapInput

// ElementType returns the element type of this Input (map[string]map[string]interface{}).
func (MapMap) ElementType() reflect.Type {
	return mapMapType
}

func (in MapMap) ToMapMapOutput() MapMapOutput {
	return ToOutput(in).(MapMapOutput)
}

func (in MapMap) ToMapMapOutputWithContext(ctx context.Context) MapMapOutput {
	return ToOutputWithContext(ctx, in).(MapMapOutput)
}

// MapMapOutput is an Output that returns map[string]map[string]interface{} values.
type MapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]interface{}).
func (MapMapOutput) ElementType() reflect.Type {
	return mapMapType
}

func (o MapMapOutput) ToMapMapOutput() MapMapOutput {
	return o
}

func (o MapMapOutput) ToMapMapOutputWithContext(ctx context.Context) MapMapOutput {
	return o
}

func (o MapMapOutput) MapIndex(k StringInput) MapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]interface{} {
		return vs[0].(map[string]map[string]interface{})[vs[1].(string)]
	}).(MapOutput)
}

var arrayArrayType = reflect.TypeOf((*[][]interface{})(nil)).Elem()

// ArrayArrayInput is an input type that accepts ArrayArray and ArrayArrayOutput values.
type ArrayArrayInput interface {
	Input

	ToArrayArrayOutput() ArrayArrayOutput
	ToArrayArrayOutputWithContext(ctx context.Context) ArrayArrayOutput
}

// ArrayArray is an input type for []ArrayInput values.
type ArrayArray []ArrayInput

// ElementType returns the element type of this Input ([][]interface{}).
func (ArrayArray) ElementType() reflect.Type {
	return arrayArrayType
}

func (in ArrayArray) ToArrayArrayOutput() ArrayArrayOutput {
	return ToOutput(in).(ArrayArrayOutput)
}

func (in ArrayArray) ToArrayArrayOutputWithContext(ctx context.Context) ArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(ArrayArrayOutput)
}

// ArrayArrayOutput is an Output that returns [][]interface{} values.
type ArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]interface{}).
func (ArrayArrayOutput) ElementType() reflect.Type {
	return arrayArrayType
}

func (o ArrayArrayOutput) ToArrayArrayOutput() ArrayArrayOutput {
	return o
}

func (o ArrayArrayOutput) ToArrayArrayOutputWithContext(ctx context.Context) ArrayArrayOutput {
	return o
}

func (o ArrayArrayOutput) Index(i IntInput) ArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []interface{} {
		return vs[0].([][]interface{})[vs[1].(int)]
	}).(ArrayOutput)
}

var intType = reflect.TypeOf((*int)(nil)).Elem()

// IntInput is an input type that accepts Int and IntOutput values.
type IntInput interface {
	Input

	ToIntOutput() IntOutput
	ToIntOutputWithContext(ctx context.Context) IntOutput

	ToIntPtrOutput() IntPtrOutput
	ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput
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

func (in Int) ToIntOutputWithContext(ctx context.Context) IntOutput {
	return ToOutputWithContext(ctx, in).(IntOutput)
}

func (in Int) ToIntPtrOutput() IntPtrOutput {
	return in.ToIntPtrOutputWithContext(context.Background())
}

func (in Int) ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput {
	return in.ToIntOutputWithContext(ctx).ToIntPtrOutputWithContext(ctx)
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

func (o IntOutput) ToIntOutputWithContext(ctx context.Context) IntOutput {
	return o
}

func (o IntOutput) ToIntPtrOutput() IntPtrOutput {
	return o.ToIntPtrOutputWithContext(context.Background())
}

func (o IntOutput) ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v int) *int {
		return &v
	}).(IntPtrOutput)
}

var intPtrType = reflect.TypeOf((**int)(nil)).Elem()

// IntPtrInput is an input type that accepts IntPtr and IntPtrOutput values.
type IntPtrInput interface {
	Input

	ToIntPtrOutput() IntPtrOutput
	ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput
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

func (in *intPtr) ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput {
	return ToOutputWithContext(ctx, in).(IntPtrOutput)
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

func (o IntPtrOutput) ToIntPtrOutputWithContext(ctx context.Context) IntPtrOutput {
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
	ToIntArrayOutputWithContext(ctx context.Context) IntArrayOutput
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

func (in IntArray) ToIntArrayOutputWithContext(ctx context.Context) IntArrayOutput {
	return ToOutputWithContext(ctx, in).(IntArrayOutput)
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

func (o IntArrayOutput) ToIntArrayOutputWithContext(ctx context.Context) IntArrayOutput {
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
	ToIntMapOutputWithContext(ctx context.Context) IntMapOutput
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

func (in IntMap) ToIntMapOutputWithContext(ctx context.Context) IntMapOutput {
	return ToOutputWithContext(ctx, in).(IntMapOutput)
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

func (o IntMapOutput) ToIntMapOutputWithContext(ctx context.Context) IntMapOutput {
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
	ToIntArrayMapOutputWithContext(ctx context.Context) IntArrayMapOutput
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

func (in IntArrayMap) ToIntArrayMapOutputWithContext(ctx context.Context) IntArrayMapOutput {
	return ToOutputWithContext(ctx, in).(IntArrayMapOutput)
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

func (o IntArrayMapOutput) ToIntArrayMapOutputWithContext(ctx context.Context) IntArrayMapOutput {
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
	ToIntMapArrayOutputWithContext(ctx context.Context) IntMapArrayOutput
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

func (in IntMapArray) ToIntMapArrayOutputWithContext(ctx context.Context) IntMapArrayOutput {
	return ToOutputWithContext(ctx, in).(IntMapArrayOutput)
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

func (o IntMapArrayOutput) ToIntMapArrayOutputWithContext(ctx context.Context) IntMapArrayOutput {
	return o
}

func (o IntMapArrayOutput) Index(i IntInput) IntMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]int {
		return vs[0].([]map[string]int)[vs[1].(int)]
	}).(IntMapOutput)
}

var intMapMapType = reflect.TypeOf((*map[string]map[string]int)(nil)).Elem()

// IntMapMapInput is an input type that accepts IntMapMap and IntMapMapOutput values.
type IntMapMapInput interface {
	Input

	ToIntMapMapOutput() IntMapMapOutput
	ToIntMapMapOutputWithContext(ctx context.Context) IntMapMapOutput
}

// IntMapMap is an input type for map[string]IntMapInput values.
type IntMapMap map[string]IntMapInput

// ElementType returns the element type of this Input (map[string]map[string]int).
func (IntMapMap) ElementType() reflect.Type {
	return intMapMapType
}

func (in IntMapMap) ToIntMapMapOutput() IntMapMapOutput {
	return ToOutput(in).(IntMapMapOutput)
}

func (in IntMapMap) ToIntMapMapOutputWithContext(ctx context.Context) IntMapMapOutput {
	return ToOutputWithContext(ctx, in).(IntMapMapOutput)
}

// IntMapMapOutput is an Output that returns map[string]map[string]int values.
type IntMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]int).
func (IntMapMapOutput) ElementType() reflect.Type {
	return intMapMapType
}

func (o IntMapMapOutput) ToIntMapMapOutput() IntMapMapOutput {
	return o
}

func (o IntMapMapOutput) ToIntMapMapOutputWithContext(ctx context.Context) IntMapMapOutput {
	return o
}

func (o IntMapMapOutput) MapIndex(k StringInput) IntMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]int {
		return vs[0].(map[string]map[string]int)[vs[1].(string)]
	}).(IntMapOutput)
}

var intArrayArrayType = reflect.TypeOf((*[][]int)(nil)).Elem()

// IntArrayArrayInput is an input type that accepts IntArrayArray and IntArrayArrayOutput values.
type IntArrayArrayInput interface {
	Input

	ToIntArrayArrayOutput() IntArrayArrayOutput
	ToIntArrayArrayOutputWithContext(ctx context.Context) IntArrayArrayOutput
}

// IntArrayArray is an input type for []IntArrayInput values.
type IntArrayArray []IntArrayInput

// ElementType returns the element type of this Input ([][]int).
func (IntArrayArray) ElementType() reflect.Type {
	return intArrayArrayType
}

func (in IntArrayArray) ToIntArrayArrayOutput() IntArrayArrayOutput {
	return ToOutput(in).(IntArrayArrayOutput)
}

func (in IntArrayArray) ToIntArrayArrayOutputWithContext(ctx context.Context) IntArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(IntArrayArrayOutput)
}

// IntArrayArrayOutput is an Output that returns [][]int values.
type IntArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]int).
func (IntArrayArrayOutput) ElementType() reflect.Type {
	return intArrayArrayType
}

func (o IntArrayArrayOutput) ToIntArrayArrayOutput() IntArrayArrayOutput {
	return o
}

func (o IntArrayArrayOutput) ToIntArrayArrayOutputWithContext(ctx context.Context) IntArrayArrayOutput {
	return o
}

func (o IntArrayArrayOutput) Index(i IntInput) IntArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []int {
		return vs[0].([][]int)[vs[1].(int)]
	}).(IntArrayOutput)
}

var stringType = reflect.TypeOf((*string)(nil)).Elem()

// StringInput is an input type that accepts String and StringOutput values.
type StringInput interface {
	Input

	ToStringOutput() StringOutput
	ToStringOutputWithContext(ctx context.Context) StringOutput

	ToStringPtrOutput() StringPtrOutput
	ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput
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

func (in String) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return ToOutputWithContext(ctx, in).(StringOutput)
}

func (in String) ToStringPtrOutput() StringPtrOutput {
	return in.ToStringPtrOutputWithContext(context.Background())
}

func (in String) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return in.ToStringOutputWithContext(ctx).ToStringPtrOutputWithContext(ctx)
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

func (o StringOutput) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return o
}

func (o StringOutput) ToStringPtrOutput() StringPtrOutput {
	return o.ToStringPtrOutputWithContext(context.Background())
}

func (o StringOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v string) *string {
		return &v
	}).(StringPtrOutput)
}

var stringPtrType = reflect.TypeOf((**string)(nil)).Elem()

// StringPtrInput is an input type that accepts StringPtr and StringPtrOutput values.
type StringPtrInput interface {
	Input

	ToStringPtrOutput() StringPtrOutput
	ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput
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

func (in *stringPtr) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
	return ToOutputWithContext(ctx, in).(StringPtrOutput)
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

func (o StringPtrOutput) ToStringPtrOutputWithContext(ctx context.Context) StringPtrOutput {
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
	ToStringArrayOutputWithContext(ctx context.Context) StringArrayOutput
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

func (in StringArray) ToStringArrayOutputWithContext(ctx context.Context) StringArrayOutput {
	return ToOutputWithContext(ctx, in).(StringArrayOutput)
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

func (o StringArrayOutput) ToStringArrayOutputWithContext(ctx context.Context) StringArrayOutput {
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
	ToStringMapOutputWithContext(ctx context.Context) StringMapOutput
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

func (in StringMap) ToStringMapOutputWithContext(ctx context.Context) StringMapOutput {
	return ToOutputWithContext(ctx, in).(StringMapOutput)
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

func (o StringMapOutput) ToStringMapOutputWithContext(ctx context.Context) StringMapOutput {
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
	ToStringArrayMapOutputWithContext(ctx context.Context) StringArrayMapOutput
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

func (in StringArrayMap) ToStringArrayMapOutputWithContext(ctx context.Context) StringArrayMapOutput {
	return ToOutputWithContext(ctx, in).(StringArrayMapOutput)
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

func (o StringArrayMapOutput) ToStringArrayMapOutputWithContext(ctx context.Context) StringArrayMapOutput {
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
	ToStringMapArrayOutputWithContext(ctx context.Context) StringMapArrayOutput
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

func (in StringMapArray) ToStringMapArrayOutputWithContext(ctx context.Context) StringMapArrayOutput {
	return ToOutputWithContext(ctx, in).(StringMapArrayOutput)
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

func (o StringMapArrayOutput) ToStringMapArrayOutputWithContext(ctx context.Context) StringMapArrayOutput {
	return o
}

func (o StringMapArrayOutput) Index(i IntInput) StringMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]string {
		return vs[0].([]map[string]string)[vs[1].(int)]
	}).(StringMapOutput)
}

var stringMapMapType = reflect.TypeOf((*map[string]map[string]string)(nil)).Elem()

// StringMapMapInput is an input type that accepts StringMapMap and StringMapMapOutput values.
type StringMapMapInput interface {
	Input

	ToStringMapMapOutput() StringMapMapOutput
	ToStringMapMapOutputWithContext(ctx context.Context) StringMapMapOutput
}

// StringMapMap is an input type for map[string]StringMapInput values.
type StringMapMap map[string]StringMapInput

// ElementType returns the element type of this Input (map[string]map[string]string).
func (StringMapMap) ElementType() reflect.Type {
	return stringMapMapType
}

func (in StringMapMap) ToStringMapMapOutput() StringMapMapOutput {
	return ToOutput(in).(StringMapMapOutput)
}

func (in StringMapMap) ToStringMapMapOutputWithContext(ctx context.Context) StringMapMapOutput {
	return ToOutputWithContext(ctx, in).(StringMapMapOutput)
}

// StringMapMapOutput is an Output that returns map[string]map[string]string values.
type StringMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]string).
func (StringMapMapOutput) ElementType() reflect.Type {
	return stringMapMapType
}

func (o StringMapMapOutput) ToStringMapMapOutput() StringMapMapOutput {
	return o
}

func (o StringMapMapOutput) ToStringMapMapOutputWithContext(ctx context.Context) StringMapMapOutput {
	return o
}

func (o StringMapMapOutput) MapIndex(k StringInput) StringMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]string {
		return vs[0].(map[string]map[string]string)[vs[1].(string)]
	}).(StringMapOutput)
}

var stringArrayArrayType = reflect.TypeOf((*[][]string)(nil)).Elem()

// StringArrayArrayInput is an input type that accepts StringArrayArray and StringArrayArrayOutput values.
type StringArrayArrayInput interface {
	Input

	ToStringArrayArrayOutput() StringArrayArrayOutput
	ToStringArrayArrayOutputWithContext(ctx context.Context) StringArrayArrayOutput
}

// StringArrayArray is an input type for []StringArrayInput values.
type StringArrayArray []StringArrayInput

// ElementType returns the element type of this Input ([][]string).
func (StringArrayArray) ElementType() reflect.Type {
	return stringArrayArrayType
}

func (in StringArrayArray) ToStringArrayArrayOutput() StringArrayArrayOutput {
	return ToOutput(in).(StringArrayArrayOutput)
}

func (in StringArrayArray) ToStringArrayArrayOutputWithContext(ctx context.Context) StringArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(StringArrayArrayOutput)
}

// StringArrayArrayOutput is an Output that returns [][]string values.
type StringArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]string).
func (StringArrayArrayOutput) ElementType() reflect.Type {
	return stringArrayArrayType
}

func (o StringArrayArrayOutput) ToStringArrayArrayOutput() StringArrayArrayOutput {
	return o
}

func (o StringArrayArrayOutput) ToStringArrayArrayOutputWithContext(ctx context.Context) StringArrayArrayOutput {
	return o
}

func (o StringArrayArrayOutput) Index(i IntInput) StringArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []string {
		return vs[0].([][]string)[vs[1].(int)]
	}).(StringArrayOutput)
}

var urnType = reflect.TypeOf((*URN)(nil)).Elem()

// URNInput is an input type that accepts URN and URNOutput values.
type URNInput interface {
	Input

	ToURNOutput() URNOutput
	ToURNOutputWithContext(ctx context.Context) URNOutput

	ToURNPtrOutput() URNPtrOutput
	ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput
}

// ElementType returns the element type of this Input (URN).
func (URN) ElementType() reflect.Type {
	return urnType
}

func (in URN) ToURNOutput() URNOutput {
	return ToOutput(in).(URNOutput)
}

func (in URN) ToURNOutputWithContext(ctx context.Context) URNOutput {
	return ToOutputWithContext(ctx, in).(URNOutput)
}

func (in URN) ToStringOutput() StringOutput {
	return in.ToStringOutputWithContext(context.Background())
}

func (in URN) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return in.ToURNOutputWithContext(ctx).ToStringOutputWithContext(ctx)
}

func (in URN) ToURNPtrOutput() URNPtrOutput {
	return in.ToURNPtrOutputWithContext(context.Background())
}

func (in URN) ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput {
	return in.ToURNOutputWithContext(ctx).ToURNPtrOutputWithContext(ctx)
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

func (o URNOutput) ToURNOutputWithContext(ctx context.Context) URNOutput {
	return o
}

func (o URNOutput) ToStringOutput() StringOutput {
	return o.ToStringOutputWithContext(context.Background())
}

func (o URNOutput) ToStringOutputWithContext(ctx context.Context) StringOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v URN) string {
		return (string)(v)
	}).(StringOutput)
}

func (o URNOutput) ToURNPtrOutput() URNPtrOutput {
	return o.ToURNPtrOutputWithContext(context.Background())
}

func (o URNOutput) ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput {
	return o.ApplyTWithContext(ctx, func(_ context.Context, v URN) *URN {
		return &v
	}).(URNPtrOutput)
}

var uRNPtrType = reflect.TypeOf((**URN)(nil)).Elem()

// URNPtrInput is an input type that accepts URNPtr and URNPtrOutput values.
type URNPtrInput interface {
	Input

	ToURNPtrOutput() URNPtrOutput
	ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput
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

func (in *urnPtr) ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput {
	return ToOutputWithContext(ctx, in).(URNPtrOutput)
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

func (o URNPtrOutput) ToURNPtrOutputWithContext(ctx context.Context) URNPtrOutput {
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
	ToURNArrayOutputWithContext(ctx context.Context) URNArrayOutput
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

func (in URNArray) ToURNArrayOutputWithContext(ctx context.Context) URNArrayOutput {
	return ToOutputWithContext(ctx, in).(URNArrayOutput)
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

func (o URNArrayOutput) ToURNArrayOutputWithContext(ctx context.Context) URNArrayOutput {
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
	ToURNMapOutputWithContext(ctx context.Context) URNMapOutput
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

func (in URNMap) ToURNMapOutputWithContext(ctx context.Context) URNMapOutput {
	return ToOutputWithContext(ctx, in).(URNMapOutput)
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

func (o URNMapOutput) ToURNMapOutputWithContext(ctx context.Context) URNMapOutput {
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
	ToURNArrayMapOutputWithContext(ctx context.Context) URNArrayMapOutput
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

func (in URNArrayMap) ToURNArrayMapOutputWithContext(ctx context.Context) URNArrayMapOutput {
	return ToOutputWithContext(ctx, in).(URNArrayMapOutput)
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

func (o URNArrayMapOutput) ToURNArrayMapOutputWithContext(ctx context.Context) URNArrayMapOutput {
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
	ToURNMapArrayOutputWithContext(ctx context.Context) URNMapArrayOutput
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

func (in URNMapArray) ToURNMapArrayOutputWithContext(ctx context.Context) URNMapArrayOutput {
	return ToOutputWithContext(ctx, in).(URNMapArrayOutput)
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

func (o URNMapArrayOutput) ToURNMapArrayOutputWithContext(ctx context.Context) URNMapArrayOutput {
	return o
}

func (o URNMapArrayOutput) Index(i IntInput) URNMapOutput {
	return All(o, i).ApplyT(func(vs []interface{}) map[string]URN {
		return vs[0].([]map[string]URN)[vs[1].(int)]
	}).(URNMapOutput)
}

var uRNMapMapType = reflect.TypeOf((*map[string]map[string]URN)(nil)).Elem()

// URNMapMapInput is an input type that accepts URNMapMap and URNMapMapOutput values.
type URNMapMapInput interface {
	Input

	ToURNMapMapOutput() URNMapMapOutput
	ToURNMapMapOutputWithContext(ctx context.Context) URNMapMapOutput
}

// URNMapMap is an input type for map[string]URNMapInput values.
type URNMapMap map[string]URNMapInput

// ElementType returns the element type of this Input (map[string]map[string]URN).
func (URNMapMap) ElementType() reflect.Type {
	return uRNMapMapType
}

func (in URNMapMap) ToURNMapMapOutput() URNMapMapOutput {
	return ToOutput(in).(URNMapMapOutput)
}

func (in URNMapMap) ToURNMapMapOutputWithContext(ctx context.Context) URNMapMapOutput {
	return ToOutputWithContext(ctx, in).(URNMapMapOutput)
}

// URNMapMapOutput is an Output that returns map[string]map[string]URN values.
type URNMapMapOutput struct{ *OutputState }

// ElementType returns the element type of this Output (map[string]map[string]URN).
func (URNMapMapOutput) ElementType() reflect.Type {
	return uRNMapMapType
}

func (o URNMapMapOutput) ToURNMapMapOutput() URNMapMapOutput {
	return o
}

func (o URNMapMapOutput) ToURNMapMapOutputWithContext(ctx context.Context) URNMapMapOutput {
	return o
}

func (o URNMapMapOutput) MapIndex(k StringInput) URNMapOutput {
	return All(o, k).ApplyT(func(vs []interface{}) map[string]URN {
		return vs[0].(map[string]map[string]URN)[vs[1].(string)]
	}).(URNMapOutput)
}

var uRNArrayArrayType = reflect.TypeOf((*[][]URN)(nil)).Elem()

// URNArrayArrayInput is an input type that accepts URNArrayArray and URNArrayArrayOutput values.
type URNArrayArrayInput interface {
	Input

	ToURNArrayArrayOutput() URNArrayArrayOutput
	ToURNArrayArrayOutputWithContext(ctx context.Context) URNArrayArrayOutput
}

// URNArrayArray is an input type for []URNArrayInput values.
type URNArrayArray []URNArrayInput

// ElementType returns the element type of this Input ([][]URN).
func (URNArrayArray) ElementType() reflect.Type {
	return uRNArrayArrayType
}

func (in URNArrayArray) ToURNArrayArrayOutput() URNArrayArrayOutput {
	return ToOutput(in).(URNArrayArrayOutput)
}

func (in URNArrayArray) ToURNArrayArrayOutputWithContext(ctx context.Context) URNArrayArrayOutput {
	return ToOutputWithContext(ctx, in).(URNArrayArrayOutput)
}

// URNArrayArrayOutput is an Output that returns [][]URN values.
type URNArrayArrayOutput struct{ *OutputState }

// ElementType returns the element type of this Output ([][]URN).
func (URNArrayArrayOutput) ElementType() reflect.Type {
	return uRNArrayArrayType
}

func (o URNArrayArrayOutput) ToURNArrayArrayOutput() URNArrayArrayOutput {
	return o
}

func (o URNArrayArrayOutput) ToURNArrayArrayOutputWithContext(ctx context.Context) URNArrayArrayOutput {
	return o
}

func (o URNArrayArrayOutput) Index(i IntInput) URNArrayOutput {
	return All(o, i).ApplyT(func(vs []interface{}) []URN {
		return vs[0].([][]URN)[vs[1].(int)]
	}).(URNArrayOutput)
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
	RegisterOutputType(ArchiveMapMapOutput{})
	RegisterOutputType(ArchiveArrayArrayOutput{})
	RegisterOutputType(AssetOutput{})
	RegisterOutputType(AssetArrayOutput{})
	RegisterOutputType(AssetMapOutput{})
	RegisterOutputType(AssetArrayMapOutput{})
	RegisterOutputType(AssetMapArrayOutput{})
	RegisterOutputType(AssetMapMapOutput{})
	RegisterOutputType(AssetArrayArrayOutput{})
	RegisterOutputType(AssetOrArchiveOutput{})
	RegisterOutputType(AssetOrArchiveArrayOutput{})
	RegisterOutputType(AssetOrArchiveMapOutput{})
	RegisterOutputType(AssetOrArchiveArrayMapOutput{})
	RegisterOutputType(AssetOrArchiveMapArrayOutput{})
	RegisterOutputType(AssetOrArchiveMapMapOutput{})
	RegisterOutputType(AssetOrArchiveArrayArrayOutput{})
	RegisterOutputType(BoolOutput{})
	RegisterOutputType(BoolPtrOutput{})
	RegisterOutputType(BoolArrayOutput{})
	RegisterOutputType(BoolMapOutput{})
	RegisterOutputType(BoolArrayMapOutput{})
	RegisterOutputType(BoolMapArrayOutput{})
	RegisterOutputType(BoolMapMapOutput{})
	RegisterOutputType(BoolArrayArrayOutput{})
	RegisterOutputType(Float64Output{})
	RegisterOutputType(Float64PtrOutput{})
	RegisterOutputType(Float64ArrayOutput{})
	RegisterOutputType(Float64MapOutput{})
	RegisterOutputType(Float64ArrayMapOutput{})
	RegisterOutputType(Float64MapArrayOutput{})
	RegisterOutputType(Float64MapMapOutput{})
	RegisterOutputType(Float64ArrayArrayOutput{})
	RegisterOutputType(IDOutput{})
	RegisterOutputType(IDPtrOutput{})
	RegisterOutputType(IDArrayOutput{})
	RegisterOutputType(IDMapOutput{})
	RegisterOutputType(IDArrayMapOutput{})
	RegisterOutputType(IDMapArrayOutput{})
	RegisterOutputType(IDMapMapOutput{})
	RegisterOutputType(IDArrayArrayOutput{})
	RegisterOutputType(ArrayOutput{})
	RegisterOutputType(MapOutput{})
	RegisterOutputType(ArrayMapOutput{})
	RegisterOutputType(MapArrayOutput{})
	RegisterOutputType(MapMapOutput{})
	RegisterOutputType(ArrayArrayOutput{})
	RegisterOutputType(IntOutput{})
	RegisterOutputType(IntPtrOutput{})
	RegisterOutputType(IntArrayOutput{})
	RegisterOutputType(IntMapOutput{})
	RegisterOutputType(IntArrayMapOutput{})
	RegisterOutputType(IntMapArrayOutput{})
	RegisterOutputType(IntMapMapOutput{})
	RegisterOutputType(IntArrayArrayOutput{})
	RegisterOutputType(StringOutput{})
	RegisterOutputType(StringPtrOutput{})
	RegisterOutputType(StringArrayOutput{})
	RegisterOutputType(StringMapOutput{})
	RegisterOutputType(StringArrayMapOutput{})
	RegisterOutputType(StringMapArrayOutput{})
	RegisterOutputType(StringMapMapOutput{})
	RegisterOutputType(StringArrayArrayOutput{})
	RegisterOutputType(URNOutput{})
	RegisterOutputType(URNPtrOutput{})
	RegisterOutputType(URNArrayOutput{})
	RegisterOutputType(URNMapOutput{})
	RegisterOutputType(URNArrayMapOutput{})
	RegisterOutputType(URNMapArrayOutput{})
	RegisterOutputType(URNMapMapOutput{})
	RegisterOutputType(URNArrayArrayOutput{})
}
