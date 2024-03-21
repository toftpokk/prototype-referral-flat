package server

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"simplemts/lib"

	"github.com/gorilla/mux"
)

type Server struct {
	service *http.Server
	Router  *mux.Router
	certStr string
	keyStr  string
	port    int
}

func NewServer(cert string, key string, ca_cert string, port int) (server Server) {
	router := mux.NewRouter()
	caCertPool, err := lib.LoadPool(ca_cert)
	if err != nil {
		log.Fatalf("Unable to create new server: %s", err)
	}
	server = Server{
		service: &http.Server{
			Handler: router,
			Addr:    fmt.Sprintf(":%d", port),
			TLSConfig: &tls.Config{
				ClientCAs:  caCertPool,
				ClientAuth: tls.RequireAndVerifyClientCert,
			},
		},
		certStr: cert,
		keyStr:  key,
		Router:  router,
		port:    port,
	}
	return
}

func (s *Server) Serve() (err error) {
	fmt.Println("Serving server at", s.port)
	return s.service.ListenAndServeTLS(s.certStr, s.keyStr)
}
