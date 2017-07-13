package integrationtesting

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/lumi/pkg/util/contract"
)

func Test_Prefixer(t *testing.T) {
	byts := make([]byte, 0, 1000)
	buf := bytes.NewBuffer(byts)
	prefixer := newPrefixer(buf, "OK: ")
	_, err := prefixer.Write([]byte("\nsadsada\n\nasdsadsa\nasdsadsa\n"))
	contract.Assert(err == nil)
	assert.Equal(t, []byte("OK: \nOK: sadsada\nOK: \nOK: asdsadsa\nOK: asdsadsa\n"), buf.Bytes())
}
