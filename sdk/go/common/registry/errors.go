// Copyright 2025, Pulumi Corporation.
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

package registry

var (
	ErrNotFound     NotFoundError
	ErrForbidden    ForbiddenError
	ErrUnauthorized UnauthorizedError
)

type NotFoundError struct{}

func (err NotFoundError) Error() string {
	return "not found"
}

type ForbiddenError struct{}

func (err ForbiddenError) Error() string {
	return "forbidden"
}

type UnauthorizedError struct{}

func (err UnauthorizedError) Error() string {
	return "unauthorized"
}
