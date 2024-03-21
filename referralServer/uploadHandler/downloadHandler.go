package uploadhandler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"

	"github.com/gorilla/mux"
)

func (rh *UploadHandler) GetFiles(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	// Syntax
	// Semantic
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Destination != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Destination mismatch: client does not have permission to get files")
		return
	}
	if referral.ReferralStatus != db.UploadComplete { // TODO send extra data, can upload again
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not get file list: referral is in state %s", referral.ReferralStatus))
		return
	}
	files, ok := rh.Database.GetFilesByReferral(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 500, "Could not get files from referralId")
		return
	}
	// send files
	type fileItem = struct {
		UploadStatus db.UploadStatus `json:"UploadStatus"`
		Name         string          `json:"Name"`
		Checksum     string          `json:"Checksum"`
	}
	response := struct {
		Files      []fileItem `json:"Files"`
		PayloadKey string     `json:"PayloadKey"`
	}{}
	for _, file := range files {
		response.Files = append(response.Files, fileItem{
			UploadStatus: file.UploadStatus,
			Name:         file.Name,
			Checksum:     file.Checksum,
		})
	}
	response.PayloadKey = referral.PayloadKey
	res, err := json.Marshal(response)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not encode response")
		return
	}
	fmt.Fprint(w, string(res))
}

func (rh *UploadHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	filename := mux.Vars(r)["filename"]
	if filename == "" {
		lib.ErrorMessageHandler(w, r, 400, "could not parse filename")
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	// Semantic
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Destination != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Destination mismatch: client does not have permission to get files")
		return
	}
	if referral.ReferralStatus != db.UploadComplete { // TODO send extra data, can upload again
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not get file: referral is in state %s", referral.ReferralStatus))
		return
	}
	_, ok = rh.Database.GetFileByReferralName(referralId, filename)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not get file")
		return
	}
	filePath := path.Join(
		rh.payloadDir,
		fmt.Sprintf("referral-%d", referralId),
		filename,
	)
	fo, err := os.Open(filePath)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not get file")
		return
	}
	w.WriteHeader(200)
	io.Copy(w, fo)
}
