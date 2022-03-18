package workspace

import (
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestProjectRuntimeInfoRoundtripYAML(t *testing.T) {
	t.Parallel()

	doTest := func(marshal func(interface{}) ([]byte, error), unmarshal func([]byte, interface{}) error) {
		// Just name
		ri := NewProjectRuntimeInfo("nodejs", nil, nil)
		byts, err := marshal(ri)
		assert.NoError(t, err)

		var riRountrip ProjectRuntimeInfo
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Nil(t, riRountrip.Version())
		assert.Nil(t, riRountrip.Options())

		// Name and version
		vers := semver.MustParse("1.0.0")
		ri = NewProjectRuntimeInfo("nodejs", &vers, nil)
		byts, err = marshal(ri)
		assert.NoError(t, err)

		riRountrip = ProjectRuntimeInfo{}
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Nil(t, riRountrip.Options())
		assert.NotNil(t, riRountrip.Version())
		assert.Equal(t, vers, *riRountrip.Version())

		// Name and options
		ri = NewProjectRuntimeInfo("nodejs", nil, map[string]interface{}{
			"typescript":   true,
			"stringOption": "hello",
		})
		byts, err = marshal(ri)
		assert.NoError(t, err)

		riRountrip = ProjectRuntimeInfo{}
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.Nil(t, riRountrip.Version())
		assert.Equal(t, true, riRountrip.Options()["typescript"])
		assert.Equal(t, "hello", riRountrip.Options()["stringOption"])

		// Name, version and options
		ri = NewProjectRuntimeInfo("nodejs", &vers, map[string]interface{}{
			"typescript":   true,
			"stringOption": "hello",
		})
		byts, err = marshal(ri)
		assert.NoError(t, err)

		riRountrip = ProjectRuntimeInfo{}
		err = unmarshal(byts, &riRountrip)
		assert.NoError(t, err)
		assert.Equal(t, "nodejs", riRountrip.Name())
		assert.NotNil(t, riRountrip.Version())
		assert.Equal(t, vers, *riRountrip.Version())
		assert.Equal(t, true, riRountrip.Options()["typescript"])
		assert.Equal(t, "hello", riRountrip.Options()["stringOption"])
	}

	doTest(yaml.Marshal, yaml.Unmarshal)
	doTest(json.Marshal, json.Unmarshal)
}
