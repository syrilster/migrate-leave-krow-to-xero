package config

import (
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Route struct {
	Path    string
	Method  string
	Handler http.HandlerFunc
}

// Server defines the server struct
type Server struct {
	router *mux.Router
}

type ServerConfigOption func(server *Server)

//NewServer creates a new server
func NewServer(options ...ServerConfigOption) *Server {
	s := &Server{
		router: mux.NewRouter().StrictSlash(true),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

func (s *Server) WithRoutes(basePath string, routes ...Route) *Server {
	sub := s.router.PathPrefix(basePath).Subrouter()
	for _, route := range routes {
		sub.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		log.WithFields(map[string]interface{}{
			"method": route.Method,
			"path":   fmt.Sprintf("%s%s", basePath, route.Path),
		}).Infof("registered path")
	}
	return s
}

//Start the server on the defined port
func (s *Server) Start(addr string, port int) {
	panic(
		http.ListenAndServe(
			fmt.Sprintf("%s:%v", addr, port),
			handlers.RecoveryHandler(
				handlers.PrintRecoveryStack(true),
				handlers.RecoveryLogger(log.New()))(s.router),
		),
	)
}
