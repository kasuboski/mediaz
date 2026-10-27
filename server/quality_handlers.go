package server

import (
	"net/http"

	"github.com/kasuboski/mediaz/pkg/manager"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// ListQualityDefinitions lists all stored quality definitions
func (s Server) ListQualityDefinitions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := s.manager.ListQualityDefinitions(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, qualityDefinitionsDTO(result))
	}
}

// GetQualityDefinition gets a quality definition by ID
func (s Server) GetQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		result, err := s.manager.GetQualityDefinition(r.Context(), id)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, qualityDefinitionDTO(result))
	}
}

// CreateQualityDefinition creates a quality definition
func (s Server) CreateQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddQualityDefinitionRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		definition, err := s.manager.AddQualityDefinition(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, qualityDefinitionDTO(definition))
	}
}

// DeleteQualityDefinition deletes a quality definition by ID from the URL
func (s Server) DeleteQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		if err := s.manager.DeleteQualityDefinition(r.Context(), manager.DeleteQualityDefinitionRequest{ID: &id}); err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, map[string]any{"id": id})
	}
}

func (s Server) UpdateQualityDefinition() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req manager.UpdateQualityDefinitionRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		definition, err := s.manager.UpdateQualityDefinition(r.Context(), id, req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, qualityDefinitionDTO(definition))
	}
}

// GetQualityProfile gets a quality profile given an id
func (s Server) GetQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		profile, err := s.manager.GetQualityProfile(r.Context(), id)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		response, err := s.profileDTO(r.Context(), profile)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

// ListQualityProfiles lists all quality profiles
func (s Server) ListQualityProfiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mediaType := r.URL.Query().Get("type")

		var profiles []sqlcdb.QualityProfile
		var err error

		switch mediaType {
		case "movie":
			profiles, err = s.manager.ListMovieQualityProfiles(r.Context())
		case "series":
			mediaType = "episode"
			profiles, err = s.manager.ListEpisodeQualityProfiles(r.Context())
		default:
			mediaType = ""
			profiles, err = s.manager.ListQualityProfiles(r.Context())
		}

		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		response, err := s.profilesDTO(r.Context(), profiles, mediaType)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

func (s Server) CreateQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req manager.AddQualityProfileRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		profile, err := s.manager.AddQualityProfile(r.Context(), req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		response, err := s.profileDTO(r.Context(), profile)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, response)
	}
}

func (s Server) UpdateQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req manager.UpdateQualityProfileRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		profile, err := s.manager.UpdateQualityProfile(r.Context(), id, req)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		response, err := s.profileDTO(r.Context(), profile)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, response)
	}
}

func (s Server) DeleteQualityProfile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		req := manager.DeleteQualityProfileRequest{
			ID: &id,
		}

		if err := s.manager.DeleteQualityProfile(r.Context(), req); err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, req)
	}
}
