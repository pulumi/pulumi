// Copyright 2016-2021, Pulumi Corporation.
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

package archive

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileExtentionSniffing(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Format(ZIPArchive), detectArchiveFormat("./some/path/my.zip"))
	assert.Equal(t, Format(TarArchive), detectArchiveFormat("./some/path/my.tar"))
	assert.Equal(t, Format(TarGZIPArchive), detectArchiveFormat("./some/path/my.tar.gz"))
	assert.Equal(t, Format(TarGZIPArchive), detectArchiveFormat("./some/path/my.tgz"))
	assert.Equal(t, Format(JARArchive), detectArchiveFormat("./some/path/my.jar"))
	assert.Equal(t, Format(NotArchive), detectArchiveFormat("./some/path/who.knows"))

	// In #2589 we had cases where a file would look like it had an longer extension, because the suffix would include
	// some stuff after a dot. i.e. we failed to treat "my.file.zip" as a ZIPArchive.
	assert.Equal(t, Format(ZIPArchive), detectArchiveFormat("./some/path/my.file.zip"))
	assert.Equal(t, Format(TarArchive), detectArchiveFormat("./some/path/my.file.tar"))
	assert.Equal(t, Format(TarGZIPArchive), detectArchiveFormat("./some/path/my.file.tar.gz"))
	assert.Equal(t, Format(TarGZIPArchive), detectArchiveFormat("./some/path/my.file.tgz"))
	assert.Equal(t, Format(JARArchive), detectArchiveFormat("./some/path/my.file.jar"))
	assert.Equal(t, Format(NotArchive), detectArchiveFormat("./some/path/who.even.knows"))
}
