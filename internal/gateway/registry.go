package gateway

import (
	"encoding/json"
	"net/http"

	"gogate/internal/registry"
)

type registerRequest struct {
	Service string `json:"service"`
	Address string `json:"address"`
}

type deregisterRequest struct {
	Service string `json:"service"`
	ID      string `json:"id"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func RegistryAPI(reg *registry.Registry) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /registry/register", func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		inst, err := reg.Register(req.Service, req.Address)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, inst)
	})

	mux.HandleFunc("POST /registry/deregister", func(w http.ResponseWriter, r *http.Request) {
		var req deregisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if !reg.Deregister(req.Service, req.ID) {
			writeError(w, http.StatusNotFound, "instance not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deregistered"})
	})

	mux.HandleFunc("GET /registry/services", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, reg.Snapshot())
	})

	return mux
}
