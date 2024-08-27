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

package pulumi

import (
	"fmt"
	"math/big"
)

// BigInt represents an input-y *big.Int.
type BigInt interface {
	BigIntInput

	isBigInt()

	// Returns the value of this BigInt as a *big.Int.
	ToBigInt() *big.Int

	// Add returns the sum x+y.
	Add(BigInt) BigInt
}

type bigInt struct {
	*big.Int
}

// NewBigInt allocates and returns a new [BigInt] set to x.
func NewBigInt(x int64) BigInt {
	return &bigInt{big.NewInt(x)}
}

// ParseBigInt interprets a string s in 10-base and returns a BigInt.
func ParseBigInt(s string) (BigInt, error) {
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid big integer string %v", s)
	}
	return &bigInt{i}, nil
}

func MustParseBigInt(s string) BigInt {
	i, err := ParseBigInt(s)
	if err != nil {
		panic(err)
	}
	return i
}

// OfBigInt returns a BigInt from a *big.Int.
func OfBigInt(i *big.Int) BigInt {
	return &bigInt{i}
}

func (i *bigInt) isBigInt() {}

func (i *bigInt) ToBigInt() *big.Int {
	return i.Int
}

func (i *bigInt) Add(y BigInt) BigInt {
	z := new(big.Int)
	yi := y.(*bigInt)
	return &bigInt{z.Add(i.Int, yi.Int)}
}
