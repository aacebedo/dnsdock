package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type HTTPServer struct {
	config *Config
	list   ServiceListProvider
	server *http.Server
	docker *DockerManager
}

func NewHTTPServer(c *Config, list ServiceListProvider, docker *DockerManager) *HTTPServer {
	s := &HTTPServer{
		config: c,
		list:   list,
		docker: docker,
	}

	router := mux.NewRouter()
	router.HandleFunc("/services", s.getServices).Methods("GET")
	router.HandleFunc("/services/{id}", s.getService).Methods("GET")
	router.HandleFunc("/services/{id}", s.addService).Methods("PUT")
	router.HandleFunc("/services/{id}", s.updateService).Methods("PATCH")
	router.HandleFunc("/services/{id}", s.removeService).Methods("DELETE")

	router.HandleFunc("/set/ttl", s.setTTL).Methods("PUT")

	router.HandleFunc("/reload", s.reloadServices).Methods("POST")
	router.HandleFunc("/reload/{id}", s.reloadService).Methods("POST")

	s.server = &http.Server{Addr: c.httpAddr, Handler: router}

	return s
}

func (s *HTTPServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *HTTPServer) getServices(w http.ResponseWriter, req *http.Request) {
	if err := json.NewEncoder(w).Encode(s.list.GetAllServices()); err != nil {
		log.Println("Error encoding: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *HTTPServer) getService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

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
		log.Println("Error: ", err)
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
		log.Println("Error decoding JSON: ", err)
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

	if service.Ip == nil {
		http.Error(w, "Property \"ip\" is required", http.StatusInternalServerError)
		return
	}

	service.Manual = true
	s.list.AddService(id, *service)
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
		log.Println("Error decoding JSON: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if ttl, ok := input["ttl"]; ok {
		if value, ok := ttl.(float64); ok {
			service.Ttl = int(value)
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

	service.Manual = true
	// todo: this probably needs to be moved. consider stop event in the
	// middle of sending PATCH. container would not be removed.
	s.list.AddService(id, service)

}

func (s *HTTPServer) setTTL(w http.ResponseWriter, req *http.Request) {
	var value int
	if err := json.NewDecoder(req.Body).Decode(&value); err != nil {
		log.Println("Error decoding value: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.config.ttl = value

}

func (s *HTTPServer) reloadService(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	id, ok := vars["id"]
	if !ok {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	service, err := s.docker.getService(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.list.AddService(id, *service)
}

func (s *HTTPServer) reloadServices(w http.ResponseWriter, req *http.Request) {
	deletes := make(map[string]bool)
	for id, service := range s.list.GetAllServices() {
		if !service.Manual {
			deletes[id] = true
		}
	}

	containers, err := s.docker.listContainers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, container := range containers {
		deletes[container.Id] = false
		service, err := s.docker.getService(container.Id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.list.AddService(container.Id, *service)
	}

	for id, delete := range deletes {
		if delete {
			s.list.RemoveService(id)
		}
	}
}
