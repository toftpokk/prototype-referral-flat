package routehandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"simplemts/lib"
	db "simplemts/lib/database"
	"simplemts/referralServer/server"
	uploadhandler "simplemts/referralServer/uploadHandler"
)

type RouteHander struct {
	Database *db.Database
}

func RegisterRoutes(server server.Server, database *db.Database) {
	// Paths
	handler := RouteHander{
		Database: database,
	}
	uploadHandler := uploadhandler.NewUploadHandler(database)
	// Create Referral
	server.Router.Use(handler.AuthenticationMiddleware)
	server.Router.HandleFunc("/", handler.CreateReferral).Methods("POST")
	// Poll incoming requests (for destination)
	server.Router.HandleFunc("/incoming", func(w http.ResponseWriter, r *http.Request) {
		handler.Poll(w, r, false)
	}).Methods("GET")
	// Poll outgoing requests (for origin)
	server.Router.HandleFunc("/outgoing", func(w http.ResponseWriter, r *http.Request) {
		handler.Poll(w, r, true)
	}).Methods("GET")

	// Frontend
	server.Router.HandleFunc("/hospitals", handler.GetHospitals).Methods("GET")
	server.Router.HandleFunc("/{referralId}", handler.GetReferral).Methods("GET")

	// Grant
	server.Router.HandleFunc("/{referralId}/grant", handler.GrantReferral).Methods("POST")
	// Upload
	server.Router.HandleFunc("/{referralId}/upload", uploadHandler.Initiate).Methods("POST")
	server.Router.HandleFunc("/{referralId}/upload/begin", uploadHandler.ChunkBegin).Methods("POST")
	server.Router.HandleFunc("/{referralId}/upload/file/{filename}/{chunkIndex}", uploadHandler.ChunkUpload).Methods("POST")
	server.Router.HandleFunc("/{referralId}/upload/complete", uploadHandler.Complete).Methods("POST")
	// server.Router.HandleFunc("/{referralId}/upload/error", uploadHandler.Error).Methods("GET")
	// Download
	server.Router.HandleFunc("/{referralId}/download", uploadHandler.GetFiles).Methods("GET")

	// server.Router.HandleFunc("/{referralId}/download/begin", downloadHandler.ChunkBegin).Methods("GET")
	server.Router.HandleFunc("/{referralId}/download/{filename}", uploadHandler.DownloadFile).Methods("GET")
	// server.Router.HandleFunc("/{referralId}/upload/complete", downloadHandler.Complete).Methods("GET")
	// server.Router.HandleFunc("/{referralId}/upload/error", downloadHandler.Error).Methods("GET")

	// Referral complete
	server.Router.HandleFunc("/{referralId}/complete", handler.Complete).Methods("POST")
}

func (rh *RouteHander) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		certs := r.TLS.PeerCertificates
		if len(certs) < 1 {
			lib.ErrorMessageHandler(w, r, 400, "Certificate Error")
			return
		}
		hospital, ok := rh.Database.GetHospitalBySerial(certs[0].SerialNumber.String())
		if !ok {
			// Certificate error
			lib.ErrorMessageHandler(w, r, 400, "Unknown hospital")
			return
		}
		rWithHospital := lib.AddHospitalContext(r, hospital.HospitalId)
		next.ServeHTTP(w, rWithHospital)
	})
}

func validateSyntaxPatient(response *db.PatientObject) error {
	// CitizenId
	// Prefix
	switch response.Prefix {
	case "mr", "mrs", "ms":
	default:
		return fmt.Errorf("prefix should be mr, mrs, ms")
	}
	// Gender
	if response.Gender != "male" && response.Gender != "female" {
		return fmt.Errorf("gender should be male or female")
	}
	// Telephone
	match, _ := regexp.MatchString(`^\d{10}$`, response.Telephone)
	if !match {
		return fmt.Errorf("telephone should have 10 digits")
	}
	return nil
}

func (rh *RouteHander) CreateReferral(w http.ResponseWriter, r *http.Request) {
	// Needs to check empty, else empty dest
	clientHospitalId := lib.GetContextHospital(r)
	if clientHospitalId == "" {
		lib.ErrorMessageHandler(w, r, 400, "Could not find client's hospital Id")
		return
	}
	response := struct {
		// Referral
		db.ReferralObject
		db.PatientObject
	}{}
	// Syntax Check
	err := lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	err = validateSyntaxPatient(&response.PatientObject)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// Semantic Check
	if clientHospitalId != response.Origin {
		lib.ErrorMessageHandler(w, r, 400, "Origin needs to be client")
		return
	}
	if response.Origin == response.Destination {
		lib.ErrorMessageHandler(w, r, 400, "Destination cannot be origin")
		return
	}
	// TODO Check certificate match origin
	// TODO Check Origin/Destination Hospital Exists
	// Work
	referral := db.Referral{
		ReferralObject: response.ReferralObject,
		PatientObject:  response.PatientObject,
	}
	id, ok := rh.Database.CreateReferralServer(referral)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not create referral")
		return
	}
	w.WriteHeader(201)
	fmt.Fprintf(w, `{"id":%d}`, id)
}

func (rh *RouteHander) Poll(w http.ResponseWriter, r *http.Request, isOrigin bool) {
	clientHospitalId := lib.GetContextHospital(r)
	var referrals []db.Referral
	if isOrigin {
		referrals = rh.Database.GetReferralsByOrigin(clientHospitalId)
	} else {
		referrals = rh.Database.GetReferralsByDestination(clientHospitalId)
	}

	type referral = struct {
		Id             int               `json:"Id" validate:"required"`
		ReferralStatus db.ReferralStatus `json:"ReferralStatus" validate:"required"`
		db.PatientObject
		Destination string
		Origin      string
		Reason      string
		Created     int64
	}
	// Work
	var referralList []referral

	for _, val := range referrals {
		referralList = append(referralList, referral{
			Id:             val.Id,
			ReferralStatus: val.ReferralStatus,
			PatientObject:  val.PatientObject,
			Created:        val.Created,
			Origin:         val.Origin,
			Destination:    val.Destination,
			Reason:         val.Reason,
		})
	}
	w.WriteHeader(200)
	if referralList == nil {
		fmt.Fprintf(w, `{"referrals":[]}`)
		return
	}

	referralJson, err := json.Marshal(referralList)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not create referral")
		return
	}
	fmt.Fprintf(w, `{"referrals":%s}`, string(referralJson))
}

func (rh *RouteHander) GrantReferral(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	response := struct {
		Granted bool `json:"Granted"` // No 'required' because can be true or false https://pkg.go.dev/github.com/go-playground/validator/v10#hdr-Required
	}{}
	// Syntax Check
	err = lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// Semantic Check
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Destination != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Destination mismatch: client does not have permission to grant referral")
		return
	}
	if referral.ReferralStatus != db.Consented {
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not grant: referral is in state %s", referral.ReferralStatus))
		return
	}
	// Work
	var update db.ReferralStatus
	if response.Granted {
		update = db.Granted
	} else {
		update = db.NotGranted
	}
	ok = rh.Database.UpdateStatusReferralById(referralId, update)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not update referral")
		return
	}
	w.WriteHeader(200)
}

func (rh *RouteHander) Complete(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	// Semantic Check
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Destination != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Destination mismatch: client does not have permission to grant referral")
		return
	}
	if referral.ReferralStatus != db.UploadComplete {
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not grant: referral is in state %s", referral.ReferralStatus))
		return
	}
	ok = rh.Database.UpdateStatusReferralById(referralId, db.Complete)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not update referral")
		return
	}
	w.WriteHeader(200)
}

func (rh *RouteHander) GetReferral(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	// Semantic
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Destination != clientHospitalId && referral.Origin != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Hospital mismatch: client does not have permission to view referral")
		return
	}
	// Work
	resultReferral := struct {
		db.ReferralObject
		db.PatientObject
		Reason         string
		Created        int64
		ReferralStatus db.ReferralStatus
	}{
		ReferralObject: referral.ReferralObject,
		PatientObject:  referral.PatientObject,
		Reason:         referral.Reason,
		Created:        referral.Created,
		ReferralStatus: referral.ReferralStatus,
	}

	w.WriteHeader(200)
	referralJson, err := json.Marshal(resultReferral)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not create referral")
		return
	}
	fmt.Fprint(w, string(referralJson))
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
