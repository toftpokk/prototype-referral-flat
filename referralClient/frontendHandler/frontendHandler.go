package frontendhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	"simplemts/referralClient/client"
	hishandler "simplemts/referralClient/hisHandler"
	"slices"
	"strings"

	"github.com/gorilla/mux"
)

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

// Something that can make requests

type RouteHander struct {
	Client    lib.Requester
	ServerURL string
	Database  *db.Database
	uploadDir string
	resultDir string
	His       *hishandler.His
}

func (frontend FrontendServer) RegisterRoutes(
	client *client.Client,
	serverURL string,
	database *db.Database,
	his *hishandler.His,
) {
	handler := RouteHander{
		Client:    client,
		ServerURL: serverURL,
		Database:  database,
		uploadDir: lib.GetEnv("ORIGIN_UPLOAD_DIR", "../../client-upload"),
		resultDir: lib.GetEnv("DEST_RESULT_DIR", "../../client-upload"),
		His:       his,
	}
	frontend.router.Use(lib.CORS)
	// Create Referral
	frontend.router.HandleFunc("/", handler.CreateReferral).Methods("POST")
	// Check Referral State
	// Get All Active/Inactive Referrals
	frontend.router.HandleFunc("/doctor", handler.ListReferralDoctor).Methods("GET")
	frontend.router.HandleFunc("/patient", handler.GetPatients).Methods("GET")
	frontend.router.HandleFunc("/patient/{patientId}/summary", handler.GetPatientDataSummary).Methods("GET")
	frontend.router.HandleFunc("/hospitals", handler.GetHospitals).Methods("GET")
	frontend.router.HandleFunc("/referral/{referralId}", handler.GetReferral).Methods("GET")
	// staff endpoints
	frontend.router.HandleFunc("/staff", handler.ListStaffReferral).Methods("GET")
	frontend.router.HandleFunc("/referral/{referralId}/grant", handler.GrantReferral).Methods("POST")
	frontend.router.HandleFunc("/referral/{referralId}/file", handler.GetFiles).Methods("GET")
	frontend.router.HandleFunc("/referral/{referralId}/outfile", handler.GetOutFiles).Methods("GET")
	frontend.router.HandleFunc("/referral/{referralId}/download/{fileName}", handler.DownloadFile).Methods("GET")
	frontend.router.HandleFunc("/assign/{referralId}", handler.AssignDoctor).Methods("POST")
	frontend.router.HandleFunc("/assign/{referralId}", handler.CheckAssign).Methods("GET")
	frontend.router.HandleFunc("/assign/{referralId}/data", handler.GetOutRefFile).Methods("GET")
}

func getItem(name string, form *multipart.Form) string {
	val := form.Value[name]
	if len(val) < 1 {
		return ""
	}
	return val[0]
}

type formData struct {
	// TODO Files
	db.ReferralObject
	db.PatientObject
	db.CreationData
}

func parseForm(form *multipart.Form) (response formData, err error) {
	// Data
	response = formData{
		ReferralObject: db.ReferralObject{
			Origin:      "",
			Destination: getItem("Destination", form),
			Department:  getItem("Department", form),
			Reason:      getItem("Reason", form),
		},
		PatientObject: db.PatientObject{
			CitizenId: getItem("CitizenId", form),
			Prefix:    getItem("Prefix", form),
			FirstName: getItem("FirstName", form),
			LastName:  getItem("LastName", form),
			BirthDate: getItem("BirthDate", form),
			Address:   getItem("Address", form),
			Gender:    getItem("Gender", form),
			Telephone: getItem("Telephone", form),
			Email:     getItem("Email", form),
		},
		CreationData: db.CreationData{
			History:   getItem("History", form),
			Diagnosis: getItem("Diagnosis", form),
		},
	}

	return
}

func (rh *RouteHander) CreateReferral(w http.ResponseWriter, r *http.Request) {
	// typ := mime.TypeByExtension(r.Header.Get("Content-Type"))
	// fmt.Println(r.Header.Get("Content-Type"), typ)
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		lib.ErrorMessageHandler(w, r, 400, "Not a form-data request")
		return
	}

	// Everything is checked at Server, so no worries
	// Parse Forms
	err := r.ParseMultipartForm(32 << 20) // max 32mb
	if err != nil {
		return
	}
	form := r.MultipartForm
	request, err := parseForm(form)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// check duplicate
	dupes := map[string]bool{}
	for _, file := range form.File["files"] {
		if _, has := dupes[file.Filename]; has {
			lib.ErrorMessageHandler(w, r, 400, fmt.Sprint("Could not upload duplicate filename ", file.Filename))
			return
		} else {
			dupes[file.Filename] = true
		}
		if file.Filename == "ReferralData.json" {
			lib.ErrorMessageHandler(w, r, 400, fmt.Sprint("Could not upload file ", file.Filename))
			return
		}
	}
	request.Origin = lib.GetEnv("HOSPITAL_ID", "1111") // Not trust frontend

	// Send data to server
	serverRequest := struct {
		db.ReferralObject
		db.PatientObject
	}{
		ReferralObject: request.ReferralObject,
		PatientObject:  request.PatientObject,
		// not send files
	}
	// Check files

	// Semantics Check
	// Work
	// TODO make chunk data
	jsonRequest, _ := json.Marshal(serverRequest)
	resp, code, err := rh.Client.MakeJsonRequest(rh.ServerURL+"/", string(jsonRequest))
	if err != nil || code != 201 {
		fmt.Println("Server-side Referral Creation Error:", resp)
		lib.ErrorMessageHandler(w, r, 500, fmt.Sprint("Could not create referral: ", code))
		return
	}
	response := struct {
		Id int `json:"id"`
	}{}

	err = json.NewDecoder(strings.NewReader(resp)).Decode(&response)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not decode server response")
		return
	}
	referralIdString := fmt.Sprint(response.Id)

	// Attachments
	attachmentList := form.Value["attachments"]
	summaryList := []hishandler.Summary{}
	for _, attachment := range attachmentList {
		summary, err := rh.His.GetSummaryWithId(request.CitizenId, attachment)
		if err != nil {
			lib.ErrorMessageHandler(w, r, 400, err.Error())
			return
		}
		summaryList = append(summaryList, summary)
	}
	f, err := lib.CreateFile(path.Join(rh.uploadDir, referralIdString, "files", "ReferralData.json"))
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// Add creationdata
	marshalData := struct {
		Summary []hishandler.Summary `json:"Summary"`
		db.CreationData
	}{
		Summary:      summaryList,
		CreationData: request.CreationData,
	}
	jsonPayload, err := json.Marshal(marshalData)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	_, err = f.Write(jsonPayload)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}

	// Files
	for _, file := range form.File["files"] {
		uploadedFile, err := file.Open()
		if err != nil {
			lib.ErrorMessageHandler(w, r, 400, err.Error())
			return
		}
		f, err := lib.CreateFile(path.Join(rh.uploadDir, referralIdString, "files", file.Filename))
		if err != nil {
			lib.ErrorMessageHandler(w, r, 400, err.Error())
			return
		}
		io.Copy(f, uploadedFile)
	}

	w.WriteHeader(201)
	fmt.Fprint(w, resp)
}

func (rh *RouteHander) CheckAssign(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could get id")
		return
	}
	_, ok := rh.Database.GetReceiptByReferral(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find assignment")
		return
	}
	w.WriteHeader(200)
}

func (rh *RouteHander) AssignDoctor(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could get id")
		return
	}
	doctor := r.URL.Query().Get("doctor")
	if doctor == "" {
		lib.ErrorMessageHandler(w, r, 400, "Could get doctor")
		return
	}
	rec := db.ReferralReceipt{
		Referral: referralId,
		DoctorId: doctor,
	}
	fmt.Println(rec)
	err = rh.Database.CreateReferralReceipt(rec)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not assign doctor")
		return
	}
	w.WriteHeader(201)
}

func (rh *RouteHander) ListReferralDoctor(w http.ResponseWriter, r *http.Request) {
	resp, code, err := rh.Client.MakeGetRequest(rh.ServerURL + "/outgoing")
	if err != nil || code != 200 {
		lib.ErrorMessageHandler(w, r, 400, "Could not get referrals")
		return
	}
	response := struct {
		Referrals []db.Referral `json:"referrals"`
	}{}
	resp1, code, err := rh.Client.MakeGetRequestRaw(rh.ServerURL + "/incoming")
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	if code != 200 {
		lib.ErrorMessageHandler(w, r, 400, "Could not get referrals")
		return
	}
	err = lib.DecodeValidate(&response, resp1)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	doctorReferrals := []db.Referral{}
	receipts := rh.Database.GetReceiptByDoctor("test")
	for _, rec := range receipts {
		idx := slices.IndexFunc(response.Referrals, func(r db.Referral) bool {
			return r.Id == rec.Referral
		})
		doctorReferrals = append(doctorReferrals, response.Referrals[idx])
	}

	jsonBody, err := json.Marshal(doctorReferrals)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}

	w.WriteHeader(200)
	fmt.Fprintf(w, `{"doctorReferrals":%s,"referrals":%s}`, jsonBody, resp)
}
func (rh *RouteHander) ListStaffReferral(w http.ResponseWriter, r *http.Request) {
	resp1, code, err := rh.Client.MakeGetRequest(rh.ServerURL + "/incoming")
	if err != nil || code != 200 {
		lib.ErrorMessageHandler(w, r, 400, "Could not get referrals")
		return
	}
	resp2, code, err := rh.Client.MakeGetRequest(rh.ServerURL + "/outgoing")
	if err != nil || code != 200 {
		lib.ErrorMessageHandler(w, r, 400, "Could not get referrals")
		return
	}
	w.WriteHeader(200)
	fmt.Fprintf(w, `{"incoming":%s,"outgoing":%s}`, resp1, resp2)
}
func (rh *RouteHander) GetReferral(w http.ResponseWriter, r *http.Request) {
	referralId := mux.Vars(r)["referralId"]
	resp, code, err := rh.Client.MakeGetRequest(rh.ServerURL + "/" + referralId)
	if err != nil || code != 200 {
		fmt.Println(resp)
		lib.ErrorMessageHandler(w, r, 500, "Could not get referrals")
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, resp)
}

func (rh *RouteHander) GetPatients(w http.ResponseWriter, r *http.Request) {
	patients := rh.His.GetPatients()
	patientJson, err := json.Marshal(patients)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not get patient from HIS")
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, (string)(patientJson))
}

func (rh *RouteHander) GetHospitals(w http.ResponseWriter, r *http.Request) {
	resp, code, err := rh.Client.MakeGetRequest(rh.ServerURL + "/hospitals")
	if err != nil || code != 200 {
		fmt.Println(resp)
		lib.ErrorMessageHandler(w, r, 500, "Could not get hospitals")
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, resp)
}

func (rh *RouteHander) GrantReferral(w http.ResponseWriter, r *http.Request) {
	// todo check if staff
	referralId := mux.Vars(r)["referralId"]
	response := struct {
		Granted bool `json:"Granted"`
	}{}
	err := lib.DecodeValidate(&response, r.Body)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	grantJson, err := json.Marshal(response)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	resp, code, err := rh.Client.MakeJsonRequest(rh.ServerURL+"/"+referralId+"/grant", (string)(grantJson))
	if err != nil || code != 200 {
		w.WriteHeader(code)
		fmt.Fprint(w, resp)
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, resp)
}
func (rh *RouteHander) GetFiles(w http.ResponseWriter, r *http.Request) {
	referralId := mux.Vars(r)["referralId"]
	files, err := os.ReadDir(path.Join(rh.resultDir, "referral-"+referralId))
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	filenames := []string{}
	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	jsonPayload, err := json.Marshal(filenames)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, string(jsonPayload))
}
func (rh *RouteHander) GetOutFiles(w http.ResponseWriter, r *http.Request) {
	referralId := mux.Vars(r)["referralId"]
	files, err := os.ReadDir(path.Join(rh.uploadDir, referralId, "files"))
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	filenames := []string{}
	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	jsonPayload, err := json.Marshal(filenames)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, string(jsonPayload))
}
func (rh *RouteHander) GetOutRefFile(w http.ResponseWriter, r *http.Request) {
	referralId := mux.Vars(r)["referralId"]
	filePath := path.Join(rh.resultDir, "referral-"+referralId, "ReferralData.json")
	f, err := os.Open(filePath)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not find file")
		return
	}
	_, err = io.Copy(w, f)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not find file")
		return
	}

}

func (rh *RouteHander) GetPatientDataSummary(w http.ResponseWriter, r *http.Request) {
	patientId := mux.Vars(r)["patientId"]
	sum, err := rh.His.GetPatientDataSummary(patientId)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, fmt.Sprintf("Could not get patient data: %s", err))
		return
	}
	sumJson, err := json.Marshal(sum)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not create payload")
		return
	}
	w.WriteHeader(200)
	fmt.Fprint(w, string(sumJson))
}

func (rh *RouteHander) DownloadFile(w http.ResponseWriter, r *http.Request) {
	referralId := mux.Vars(r)["referralId"]
	fileName := mux.Vars(r)["fileName"]
	filePath := path.Join(rh.resultDir, "referral-"+referralId, fileName)
	f, err := os.Open(filePath)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not find file")
		return
	}
	w.Header().Add("Content-Disposition", "attachment")
	_, err = io.Copy(w, f)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 500, "Could not find file")
		return
	}
}

func (s *FrontendServer) Serve() (err error) {
	fmt.Println("Serving frontend at", s.port)
	return s.service.ListenAndServe()
}
