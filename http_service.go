package roommatch

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Service struct {
	Matches *Matchmaker
	Infrai  *Client
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /admin/tenants/{tenant}", s.putTenant)
	mux.HandleFunc("PUT /admin/tenants/{tenant}/accounts/{account}", s.putAccount)
	mux.HandleFunc("POST /matchmaking/tickets", s.postTicket)
	return mux
}

func (s *Service) putTenant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Status != "active" && input.Status != "suspended" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be active or suspended"})
		return
	}
	s.Matches.SetTenant(r.PathValue("tenant"), input.Status == "active")
	writeJSON(w, http.StatusOK, map[string]string{"tenant_id": r.PathValue("tenant"), "status": input.Status})
}

func (s *Service) putAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Status != "active" && input.Status != "suspended" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be active or suspended"})
		return
	}
	if err := s.Matches.SetAccount(r.PathValue("tenant"), r.PathValue("account"), input.Status == "active"); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"account_id": r.PathValue("account"), "status": input.Status})
}

func (s *Service) postTicket(w http.ResponseWriter, r *http.Request) {
	var ticket Ticket
	if !decode(w, r, &ticket) {
		return
	}
	match, err := s.Matches.Enqueue(ticket)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if match == nil {
		writeJSON(w, http.StatusAccepted, map[string]string{"state": "waiting"})
		return
	}
	room, err := s.Infrai.ProvisionMatch(r.Context(), *match)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Code, "message": apiErr.Message})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "room provisioning failed"})
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeDomainError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, ErrTenantUnknown) {
		status = http.StatusNotFound
	}
	if errors.Is(err, ErrTenantInactive) || errors.Is(err, ErrAccountInactive) {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": strings.ReplaceAll(err.Error(), " ", "_")})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
