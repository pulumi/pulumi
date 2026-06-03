// Copyright 2023, Pulumi Corporation.
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

package util

import "testing"

func TestJoinKey(t *testing.T) {
	type args struct {
		root string
		k    string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty root",
			args: args{root: "", k: "foo"},
			want: "foo",
		},
		{
			name: "empty key",
			args: args{root: "foo", k: ""},
			want: "foo",
		},
		{
			name: "simple key",
			args: args{root: "foo", k: "bar"},
			want: "foo.bar",
		},
		{
			name: "key with dot",
			args: args{root: "foo", k: "bar.baz"},
			want: `foo["bar.baz"]`,
		},
		{
			name: "key with quote",
			args: args{root: "foo", k: "bar\"baz"},
			want: `foo["bar\"baz"]`,
		},
		{
			name: "key with quote and dot",
			args: args{root: "foo", k: "bar\"baz.qux"},
			want: `foo["bar\"baz.qux"]`,
		},
		{
			name: "key with quote and dot and backslash",
			args: args{root: "foo", k: "bar\"baz.qux\\quux"},
			want: `foo["bar\"baz.qux\quux"]`,
		},
		{
			name: "key with digit",
			args: args{root: "foo", k: "bar1"},
			want: "foo.bar1",
		},
		{
			name: "key with digit at start",
			args: args{root: "foo", k: "0bar"},
			want: `foo["0bar"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := JoinKey(tt.args.root, tt.args.k); got != tt.want {
				t.Errorf("JoinKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
