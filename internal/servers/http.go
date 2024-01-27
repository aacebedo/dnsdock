/* http.go
 *
 * Copyright (C) 2016 Alexandre ACEBEDO
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file for details.
 */

package servers

import (
	"encoding/json"
	"github.com/aacebedo/dnsdock/internal/utils"
	"github.com/gorilla/mux"
	"net/http"
)

// HTTPServer represents the http endpoint
type HTTPServer struct {
	config *utils.Config
	list   ServiceListProvider
	server *http.Server
}

// NewHTTPServer create a new http endpoint
func NewHTTPServer(c *utils.Config, list ServiceListProvider) *HTTPServer {
	s := &HTTPServer{
		config: c,
		list:   list,
	}

	router := mux.NewRouter()
	router.HandleFunc("/services", s.getServices).Methods("GET")
	router.HandleFunc("/services/{id}", s.getService).Methods("GET")
	router.HandleFunc("/services/{id}", s.addService).Methods("PUT")
	router.HandleFunc("/services/{id}", s.updateService).Methods("PATCH")
	router.HandleFunc("/services/{id}", s.removeService).Methods("DELETE")

	router.HandleFunc("/set/ttl", s.setTTL).Methods("PUT")

	s.server = &http.Server{Addr: c.HttpAddr, Handler: router}

	return s
}

// Start starts the http endpoint
func (s *HTTPServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *HTTPServer) getServices(w http.ResponseWriter, req *http.Request) {

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	if err := json.NewEncoder(w).Encode(s.list.GetAllServices()); err != nil {
		logger.Errorf("Encoding error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *HTTPServer) getService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	id, ok := vars["id"]
	if !ok {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	service, err := s.list.GetService(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(service); err != nil {
		logger.Errorf("Encoding error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *HTTPServer) addService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	id, ok := vars["id"]
	if !ok {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	service := NewService()
	if err := json.NewDecoder(req.Body).Decode(&service); err != nil {
		logger.Errorf("JSON decoding error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if service.Name == "" {
		http.Error(w, "Property \"name\" is required", http.StatusInternalServerError)
		return
	}

	if service.Image == "" {
		http.Error(w, "Property \"image\" is required", http.StatusInternalServerError)
		return
	}

	if len(service.IPs) == 0 || service.IPs[0] == nil {
		http.Error(w, "Property \"ip\" is required", http.StatusInternalServerError)
		return
	}

	err := s.list.AddService(id, *service)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
}

func (s *HTTPServer) removeService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	id, ok := vars["id"]
	if !ok {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := s.list.RemoveService(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

}

func (s *HTTPServer) updateService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	id, ok := vars["id"]
	if !ok {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	service, err := s.list.GetService(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
		logger.Errorf("JSON decoding error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ttl, ok := input["ttl"]; ok {
		if value, ok := ttl.(float64); ok {
			service.TTL = int(value)
		}
	}

	if name, ok := input["name"]; ok {
		if value, ok := name.(string); ok {
			service.Name = value
		}
	}

	if image, ok := input["image"]; ok {
		if value, ok := image.(string); ok {
			service.Image = value
		}
	}

	if image, ok := input["alias"]; ok {
		if value, ok := image.([]string); ok {
			service.Aliases = value
		}
	}

	// todo: this probably needs to be moved. consider stop event in the
	// middle of sending PATCH. container would not be removed.
	err = s.list.AddService(id, service)
  if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
  }
}

func (s *HTTPServer) setTTL(w http.ResponseWriter, req *http.Request) {
	var value int
	if err := json.NewDecoder(req.Body).Decode(&value); err != nil {
		logger.Errorf("Decoding error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.config.Ttl = value

}
