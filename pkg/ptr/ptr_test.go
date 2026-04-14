package ptr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTo(t *testing.T) {
	t.Run("test to", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, &s, To(s))
	})
}
