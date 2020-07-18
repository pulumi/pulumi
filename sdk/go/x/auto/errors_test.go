package auto

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConflicError(t *testing.T) {
	sName := fmt.Sprintf("int_test%d", rangeIn(10000000, 99999999))
	ps := ProjectSpec{
		Name:       "compilation_error",
		SourcePath: filepath.Join(".", "test", "errors", "compilation_error"),
	}
	ss := StackSpec{
		Name:    sName,
		Project: ps,
	}

	// initialize
	s, err := NewStack(ss)
	if err != nil {
		t.Errorf("failed to initialize stack, err: %v", err)
		t.FailNow()
	}

	c := make(chan error)

	// parallel updates to cause conflict
	for i := 0; i < 5; i++ {
		go func() {
			_, err := s.Up()
			c <- err
		}()
	}

	conflicts := 0

	var errs []error

	for i := 0; i < 5; i++ {
		err := <-c
		if IsConflict(err) {
			errs = append(errs, err)
			conflicts++
		}
	}

	// -- pulumi destroy --

	dRes, err := s.Destroy()
	if err != nil {
		t.Errorf("destroy failed, err: %v", err)
		t.FailNow()
	}

	assert.Equal(t, "destroy", dRes.Summary.Kind)
	assert.Equal(t, "succeeded", dRes.Summary.Result)

	// -- pulumi stack rm --

	err = s.Remove()
	assert.Nil(t, err, "failed to remove stack. Resources have leaked.")

	// should have at least one conflict
	assert.Greater(t, conflicts, 0)
}
