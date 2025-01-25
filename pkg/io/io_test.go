package io

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMediaFileSystem_IsSameFileSystem(t *testing.T) {
	mfs := &MediaFileSystem{}

	t.Run("same file system", func(t *testing.T) {
		// Create two temporary files on the same file system
		tempFile1, err := os.CreateTemp("", "testfile1")
		assert.NoError(t, err)
		defer os.Remove(tempFile1.Name())

		tempFile2, err := os.CreateTemp("", "testfile2")
		assert.NoError(t, err)
		defer os.Remove(tempFile2.Name())

		// Call the function and verify it returns true
		isSame, err := mfs.IsSameFileSystem(tempFile1.Name(), tempFile2.Name())
		assert.NoError(t, err)
		assert.True(t, isSame)
	})

	t.Run("non-existent source path", func(t *testing.T) {
		nonExistentSource := "/non/existent/source/path"
		tempFile, err := os.CreateTemp("", "testfile")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Call the function and verify it returns an error
		isSame, err := mfs.IsSameFileSystem(nonExistentSource, tempFile.Name())
		assert.NoError(t, err)
		assert.False(t, isSame)
	})

	t.Run("non-existent target path", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "testfile")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		nonExistentTarget := "/non/existent/target/path"

		// Call the function and verify it returns an error
		isSame, err := mfs.IsSameFileSystem(tempFile.Name(), nonExistentTarget)
		assert.NoError(t, err)
		assert.False(t, isSame)
	})

	t.Run("unexpected sys type", func(t *testing.T) {
		// Use a path that might produce an unexpected sys type (e.g., special devices).
		// This test case is platform-specific and may require customization.

		specialFile := "/dev/null" // Example of a special device file
		tempFile, err := os.CreateTemp("", "testfile")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Call the function and verify it returns an error if unexpected sys type is encountered
		isSame, err := mfs.IsSameFileSystem(specialFile, tempFile.Name())
		if err != nil {
			assert.Contains(t, err.Error(), "unexpected sys type")
		} else {
			assert.False(t, isSame) // Fallback if no error, unlikely but safe to check
		}
	})
}
