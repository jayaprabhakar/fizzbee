package modelchecker

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestRemoveCurrentThread is a unit test for Process.removeCurrentThread.
func TestRemoveCurrentThread(t *testing.T) {
	p := &Process{
		Threads: []*Thread{
			&Thread{},
			&Thread{},
			&Thread{},
		},
		current: 1,
	}
	p.removeCurrentThread()
	assert.Equal(t, 2, len(p.Threads))
	assert.Equal(t, 0, p.current)

	p.current = 1
	p.removeCurrentThread()
	assert.Equal(t, 1, len(p.Threads))
	assert.Equal(t, 0, p.current)

	p.current = 0
	p.removeCurrentThread()
	assert.Equal(t, 0, len(p.Threads))
	assert.Equal(t, 0, p.current)
}
