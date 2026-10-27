package server

import (
	"net/http"

	"github.com/kasuboski/mediaz/pkg/manager"
)

// ListDownloadClients lists all download clients
func (s Server) ListDownloadClients() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients, err := s.manager.ListDownloadClients(r.Context())
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, downloadClientsDTO(clients))
	}
}

// GetDownloadClient gets a download client by ID
func (s Server) GetDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		client, err := s.manager.GetDownloadClient(r.Context(), id)
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, downloadClientDTO(client))
	}
}

// CreateDownloadClient stores a download client
func (s Server) CreateDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req downloadClientRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		result, err := s.manager.CreateDownloadClient(r.Context(), manager.AddDownloadClientRequest{DownloadClient: req.row()})
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusCreated, downloadClientDTO(result))
	}
}

func (s Server) UpdateDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		var req downloadClientRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		result, err := s.manager.UpdateDownloadClient(r.Context(), id, manager.UpdateDownloadClientRequest{DownloadClient: req.row()})
		if err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, downloadClientDTO(result))
	}
}

func (s Server) DeleteDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.parseURLInt64(w, r, "id")
		if !ok {
			return
		}

		if err := s.manager.DeleteDownloadClient(r.Context(), id); err != nil {
			s.respondError(r, w, http.StatusInternalServerError, err)
			return
		}
		s.respond(r, w, http.StatusOK, id)
	}
}

func (s Server) TestDownloadClient() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req downloadClientRequest
		if !s.decodeJSON(w, r, &req) {
			return
		}

		if err := s.manager.TestDownloadClient(r.Context(), manager.AddDownloadClientRequest{DownloadClient: req.row()}); err != nil {
			s.respondError(r, w, http.StatusBadRequest, err)
			return
		}

		s.respond(r, w, http.StatusOK, map[string]string{"message": "Connection successful"})
	}
}
