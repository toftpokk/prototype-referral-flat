package frontendhandler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"simplemts/lib"
	db "simplemts/lib/database"

	"github.com/gorilla/mux"
)

// for patients
type FrontendServer struct {
	service *http.Server
	router  *mux.Router
	port    int
}

func NewFrontend(port int) (frontendServer FrontendServer) {
	router := mux.NewRouter()
	frontendServer = FrontendServer{
		service: &http.Server{
			Handler: router,
			Addr:    fmt.Sprintf(":%d", port),
		},
		router: router,
		port:   port,
	}
	return
}

type RouteHander struct {
	Database  *db.Database
	ServerURL string
}

func (frontend FrontendServer) RegisterRoutes(database *db.Database) {
	handler := RouteHander{
		Database: database,
	}
	frontend.router.Use(lib.CORS)
	// Create Referral
	// frontend.router.HandleFunc("/", handler.CreateReferral).Methods("POST")
	// // Check Referral State
	// // Get All Active/Inactive Referrals
	frontend.router.HandleFunc("/", handler.ListReferral).Methods("GET")
	frontend.router.HandleFunc("/register", handler.CreatePatient).Methods("POST")
	frontend.router.HandleFunc("/login", handler.Login).Methods("POST")
	frontend.router.HandleFunc("/referral/{referralId}", handler.GetReferral).Methods("GET")
	// frontend.router.HandleFunc("/patients", handler.GetPatients).Methods("GET")
	frontend.router.HandleFunc("/hospitals", handler.GetHospitals).Methods("GET")
	frontend.router.HandleFunc("/hospital", handler.CreateHospital).Methods("POST")
	frontend.router.HandleFunc("/referral/{referralId}/consent", handler.GiveConsent).Methods("POST")
}

func (rh *RouteHander) ListReferral(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	patient, ok := rh.Database.GetPatientByUsername(username)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not find patient")
		return
	}
	referralList := rh.Database.GetReferralsByPatient(patient.CitizenId)
	w.WriteHeader(200)
	referralJson, err := json.Marshal(referralList)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not create referral list")
		return
	}
	fmt.Fprintf(w, `{"referrals":%s}`, string(referralJson))
}

func (rh *RouteHander) GetReferral(w http.ResponseWriter, r *http.Request) {
	// todo patientId
	// citizenId := "12312312312312"
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// todo patientId
	referrals, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}

	w.WriteHeader(200)
	referralJson, err := json.Marshal(referrals)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not create referral list")
		return
	}
	fmt.Fprint(w, string(referralJson))
}

func (rh *RouteHander) GiveConsent(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	patient, ok := rh.Database.GetPatientByUsername(username)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not find patient")
		return
	}
	citizenId := patient.CitizenId
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// todo patientId
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.CitizenId != citizenId {
		lib.ErrorMessageHandler(w, r, 400, "Not allowed to give consent")
		return
	}
	ok = rh.Database.UpdateStatusReferralById(referralId, db.Consented)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not give consent")
		return
	}

	w.WriteHeader(200)
}

func (rh *RouteHander) GetHospitals(w http.ResponseWriter, r *http.Request) {
	hospitals, ok := rh.Database.GetHospitals()
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not find hospitals")
		return
	}
	// Work
	hospitalsJson, err := json.Marshal(hospitals)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not encode hospitals")
		return
	}
	fmt.Fprint(w, string(hospitalsJson))
}

func genKey(hospitalName string) (err error) {
	// https://stackoverflow.com/questions/70254968/create-key-and-certificate-in-golang-same-as-openssl-do-for-local-host
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}
	// PEM encode
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: keyBytes,
		},
	)
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * 10 * time.Hour)
	template := x509.Certificate{
		SerialNumber: big.NewInt(0),
		Subject: pkix.Name{
			CommonName:    hospitalName,
			Organization:  []string{hospitalName},
			StreetAddress: []string{"Bangkok"},
			Country:       []string{"Thailand"},
		},
		Extensions: []pkix.Extension{
			{
				Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
				Critical: false,
				Value:    []byte(hospitalName),
			},
		},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	// Create cert
	// derBytes, err := x509.CreateCertificate(rand.Reader, &template, ca, &key.PublicKey, caPrvKey)
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return
	}
	// lib.LoadCert()
	certPem := string(pem.EncodeToMemory(
		&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		},
	))
	fmt.Println(string(keyPEM), certPem)
	return
}

func (rh *RouteHander) CreatePatient(w http.ResponseWriter, r *http.Request) {
	response := struct {
		Username  string `json:"Username" validate:"required"`
		Password  string `json:"Password" validate:"required"`
		Email     string `json:"Email" validate:"required"`
		CitizenId string `json:"CitizenId" validate:"required"`
	}{}
	err := lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}

	// Check existing patient
	_, ok := rh.Database.GetPatientByUsername(response.Username)
	if ok {
		lib.ErrorMessageHandler(w, r, 400, "user already exists")
		return
	}
	_, ok = rh.Database.GetPatientByCitizenId(response.CitizenId)
	if ok {
		lib.ErrorMessageHandler(w, r, 400, "user already exists")
		return
	}

	// jsonBody := []byte(fmt.Sprintf(`{
	// 	"namespace":"citizen_id",
	// 	"identifier":"%s",
	// 	"min_idp":"",
	// 	"withMockData":false,
	// 	"request_timeout":"",
	// 	"idp_id_list":[],
	// 	"data_request_list":[],
	// 	"mode":2
	// }`, lib.GetEnv("NDID_IDENTIFIER", "a")))
	// bodyReader := bytes.NewReader(jsonBody)
	// resp, err := http.Post("http://localhost:9000/createRequest", "application/json", bodyReader)
	// if err != nil {
	// 	lib.ErrorMessageHandler(w, r, 400, err.Error())
	// 	return
	// }

	// ndidresp := struct {
	// 	RequestId   string `json:"requestId" validate:"required"`
	// 	ReferenceId string `json:"referenceId" validate:"required"`
	// }{}
	// err = lib.DecodeValidate(&ndidresp, resp.Body)
	// if err != nil {
	// 	lib.ErrorMessageHandler(w, r, 400, err.Error())
	// 	return
	// }
	hashedPassword, err := hashPassword(response.Password)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "could not hash password")
		return
	}
	patient := db.Patient{
		CitizenId:  response.CitizenId,
		Username:   response.Username,
		Password:   hashedPassword,
		IsVerified: true,
	}
	rh.Database.CreatePatient(patient)
	// requestId referenceId
	w.WriteHeader(201)
	fmt.Fprintf(w, `{"id":"%s"}`, "12345")

	// curl 'http://localhost:9000/createRequest' -X POST -H 'User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/116.0' -H 'Accept: application/json' -H 'Accept-Language: en-US,en;q=0.5' -H 'Accept-Encoding: gzip, deflate, br' -H 'Referer: http://localhost:9000/' -H 'Content-Type: application/json' -H 'Origin: http://localhost:9000' -H 'DNT: 1' -H 'Connection: keep-alive' -H 'Cookie: immich_access_token=WcfeOdFXG05kLNqITEsFl7gvzU7UPKWEJIPHJVTms; immich_auth_type=password; csrftoken=OL97m8P7iKsPpU2wCzcqxQAGnnNKxrBp; username=test; role=doctor; io=ASiHPGczQcfIH7zpAAAB' -H 'Sec-Fetch-Dest: empty' -H 'Sec-Fetch-Mode: cors' -H 'Sec-Fetch-Site: same-origin' -H 'Pragma: no-cache' -H 'Cache-Control: no-cache' --data-raw '{"namespace":"citizen_id","identifier":"test","min_idp":"","withMockData":false,"request_timeout":"","idp_id_list":[],"data_request_list":[],"mode":2}'
}

func (rh *RouteHander) Login(w http.ResponseWriter, r *http.Request) {
	response := struct {
		Username string `json:"Username" validate:"required"`
		Password string `json:"Password" validate:"required"`
	}{}
	err := lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	patient, ok := rh.Database.GetPatientByUsername(response.Username)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not find user")
		return
	}
	// patient.Password
	doPasswordsMatch(patient.Password, response.Password)
	w.WriteHeader(200)
}

func (rh *RouteHander) CreateHospital(w http.ResponseWriter, r *http.Request) {
	// Create hospital
	response := db.Hospital{}
	err := lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}

	// Generate key
	response.CertSerial = ""
	// c := "openssl"
	// a := []string{"dgst", "-sha512", "-sign", "signature.key", "-out", "FileOut.signed", "FileToBeSigned.txt"}
	// cmd := exec.Command(c, a...)
	genKey(response.HospitalName)
	if err == nil {
		lib.ErrorMessageHandler(w, r, 400, `{a:"aaaaaaa"}`)
		return
	}
	// add to database
	id, ok := rh.Database.ServerCreateHospital(response)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not create hospital")
		return
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, `{"id":%d}`, id)
	// fmt.Fprint(w, string(hospitalsJson))
	// TODO client_key
	// return public & private key, id
}

func (s *FrontendServer) Serve() (err error) {
	fmt.Println("Serving frontend at", s.port)
	return s.service.ListenAndServe()
}
