package main

import (
	"fmt"
	"log"
	"path"
	lib "simplemts/lib"
	db "simplemts/lib/database"
	"simplemts/referralClient/client"
	frontendhandler "simplemts/referralClient/frontendHandler"
	hishandler "simplemts/referralClient/hisHandler"
	pollinghandler "simplemts/referralClient/pollingHandler"

	"github.com/joho/godotenv"
)

func testConnection(c *client.Client, serverURL string) {
	fmt.Println("Testing Connection to Server...")
	resp, code, err := c.MakeGetRequest(serverURL + "/hospitals")
	if err != nil {
		log.Fatal("Server Connection Error:", err)
	}
	if code != 200 {
		log.Fatal("Server Connection Error: ", code, resp)
	}
	fmt.Println("Connected!")
}

func main() {
	// TODO register node
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	serverPort := lib.GetEnvAsInt("SERVER_PORT", 8443)
	serverPath := lib.GetEnv("SERVER_PATH", "https://localhost")
	serverURL := fmt.Sprintf("%s:%d", serverPath, serverPort)
	frontendPort := lib.GetEnvAsInt("CLIENT_FRONTEND_PORT", 8444)

	dbname := lib.GetEnv("CLIENT_DB", "referralClient.sqlite")
	database := db.NewDatabase(dbname)

	authDir := lib.GetEnv("AUTH_DIR", "./auth")
	keyFile := lib.GetEnv("KEY_FILE", "./origin.key")
	certFile := lib.GetEnv("CERT_FILE", "./origin.crt")
	caFile := lib.GetEnv("CA_FILE", "./ca.crt")
	fmt.Println(certFile)

	key := path.Join(authDir, keyFile)
	cert := path.Join(authDir, certFile)
	caCert := path.Join(authDir, caFile)

	client := client.NewClient(cert, key, caCert)
	frontend := frontendhandler.NewFrontend(frontendPort)
	his := hishandler.InitiateConnection()

	testConnection(&client, serverURL)

	frontend.RegisterRoutes(&client, serverURL, &database, &his)
	// duration_min := 5
	// polling := pollinghandler.NewPollingHandler(duration_min*60, &client, &database, serverURL)
	polling := pollinghandler.NewPollingHandler(5, &client, &database, serverURL)

	go polling.Run()

	frontendErrors := make(chan error)
	go func() {
		frontendErrors <- frontend.Serve()
	}()
	err = <-frontendErrors
	if err != nil {
		fmt.Printf("Frontend terminated with error: %s\n", err)
	}

}
