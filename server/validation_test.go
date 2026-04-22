package server

import (
	"testing"

	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/stretchr/testify/assert"
)

func TestValidation_AddMovieRequest(t *testing.T) {
	v := newTestServer()

	t.Run("empty request fails validation", func(t *testing.T) {
		req := manager.AddMovieRequest{}
		err := v.validate.Struct(req)
		assert.Error(t, err)
	})

	t.Run("valid request passes validation", func(t *testing.T) {
		req := manager.AddMovieRequest{TMDBID: 1, QualityProfileID: 1}
		err := v.validate.Struct(req)
		assert.NoError(t, err)
	})

	t.Run("zero TMDBID fails validation", func(t *testing.T) {
		req := manager.AddMovieRequest{TMDBID: 0, QualityProfileID: 1}
		err := v.validate.Struct(req)
		assert.Error(t, err)
	})
}

func TestValidation_TriggerJobRequest(t *testing.T) {
	v := newTestServer()

	t.Run("empty type fails validation", func(t *testing.T) {
		req := manager.TriggerJobRequest{}
		err := v.validate.Struct(req)
		assert.Error(t, err)
	})

	t.Run("valid type passes validation", func(t *testing.T) {
		req := manager.TriggerJobRequest{Type: "MovieIndex"}
		err := v.validate.Struct(req)
		assert.NoError(t, err)
	})
}

func TestValidation_AddQualityProfileRequest(t *testing.T) {
	v := newTestServer()

	t.Run("empty request fails validation", func(t *testing.T) {
		req := manager.AddQualityProfileRequest{}
		err := v.validate.Struct(req)
		assert.Error(t, err)
	})

	t.Run("missing quality IDs fails validation", func(t *testing.T) {
		req := manager.AddQualityProfileRequest{Name: "test", QualityIDs: []int32{}}
		err := v.validate.Struct(req)
		assert.Error(t, err)
	})

	t.Run("valid request passes validation", func(t *testing.T) {
		req := manager.AddQualityProfileRequest{Name: "test", QualityIDs: []int32{1}}
		err := v.validate.Struct(req)
		assert.NoError(t, err)
	})
}

func TestValidation_AddDownloadClientRequest(t *testing.T) {
	v := newTestServer()

	t.Run("empty request passes validation (no tags on embedded model)", func(t *testing.T) {
		req := manager.AddDownloadClientRequest{}
		err := v.validate.Struct(req)
		assert.NoError(t, err)
	})
}
