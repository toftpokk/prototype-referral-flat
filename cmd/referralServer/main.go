package main

import (
	"fmt"
	"log"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	frontendhandler "simplemts/referralServer/frontendHandler"
	routehandler "simplemts/referralServer/routeHandler"
	"simplemts/referralServer/server"

	"github.com/joho/godotenv"
)

func main() {
	// Environmental variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	serverPort := lib.GetEnvAsInt("SERVER_PORT", 8443)
	serverFrontendPort := lib.GetEnvAsInt("SERVER_FRONTEND_PORT", 8445)

	dbname := lib.GetEnv("SERVER_DB", "referralServer.sqlite")

	authDir := lib.GetEnv("AUTH_DIR", "./auth")
	keyFile := lib.GetEnv("KEY_FILE", "./central.key")
	certFile := lib.GetEnv("CERT_FILE", "./central.crt")
	caFile := lib.GetEnv("CA_FILE", "./ca.crt")

	key := path.Join(authDir, keyFile)
	cert := path.Join(authDir, certFile)
	caCert := path.Join(authDir, caFile)
	server := server.NewServer(cert, key, caCert, serverPort)
	frontend := frontendhandler.NewFrontend(serverFrontendPort)

	database := db.NewDatabase(dbname)
	routehandler.RegisterRoutes(server, &database)
	frontend.RegisterRoutes(&database)

	frontendErrors := make(chan error)
	go func() {
		frontendErrors <- frontend.Serve()
	}()
	serverErrors := make(chan error)
	go func() {
		serverErrors <- server.Serve()
	}()
	err = <-serverErrors
	if err != nil {
		fmt.Printf("Server terminated with error: %s\n", err)
	}
}
