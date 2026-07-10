// Copyright 2026, Pulumi Corporation.
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

package noderesolver

// PinnedVersion is the Node.js LTS release used when no ambient node is found.
//
// To bump: set the new LTS version (no leading v) and refresh every entry in
// pinnedChecksums from https://nodejs.org/dist/v<version>/SHASUMS256.txt.
const PinnedVersion = "24.18.0"

var pinnedChecksums = map[string]string{
	"node-v24.18.0-linux-x64.tar.gz":    "783130984963db7ba9cbd01089eaf2c2efb055c7c1693c943174b967b3050cb8",
	"node-v24.18.0-linux-arm64.tar.gz":  "6b4484c2190274175df9aa8f28e2d758a819cb1c1fe6ab481e2f95b463ab8508",
	"node-v24.18.0-darwin-x64.tar.gz":   "dfd0dbd3e721503434df7b7205e719f61b3a3a31b2bcf9729b8b91fea240f080",
	"node-v24.18.0-darwin-arm64.tar.gz": "e1a97e14c99c803e96c7339403282ea05a499c32f8d83defe9ef5ec66f979ed1",
	"node-v24.18.0-win-x64.zip":         "0ae68406b42d7725661da979b1403ec9926da205c6770827f33aac9d8f26e821",
}
