package uploadhandler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	"strconv"

	"github.com/gorilla/mux"
)

type UploadHandler struct {
	Database             *db.Database
	payloadDir           string
	chunkDir             string
	chunkTrackingList    map[int](map[string]ChunkFile)
	downloadTrackingList map[int](map[string]ChunkFile)
}

type ChunkFile = db.ChunkFile

type Chunk = db.Chunk

const (
	Incomplete = db.IncompleteChunk
	Complete   = db.CompleteChunk
)

func NewUploadHandler(database *db.Database) UploadHandler {
	return UploadHandler{
		Database:             database,
		payloadDir:           lib.GetEnv("SERVER_PAYLOAD_DIR", "../../upload"),
		chunkDir:             lib.GetEnv("SERVER_CHUNK_DIR", "../../chunk"),
		chunkTrackingList:    make(map[int](map[string]ChunkFile)),
		downloadTrackingList: make(map[int](map[string]ChunkFile)),
	}
}

func (rh *UploadHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	clientHospitalId := lib.GetContextHospital(r)
	response := struct {
		PayloadKey string          `json:"PayloadKey" validate:"required"`
		Files      []db.FileObject `json:"Files" validate:"required,unique=Name,dive"`
	}{}
	// Syntax check
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
	if referral.Origin != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Origin mismatch: client does not have permission to grant referral")
		return
	}
	if referral.ReferralStatus != db.Granted { // TODO send extra data, can upload again
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not set to upload incomplete: referral is in state %s", referral.ReferralStatus))
		return
	}
	parentPath := path.Join(rh.payloadDir, fmt.Sprint(referralId)) // file exists in /upload/referralId/fileId
	for _, file := range response.Files {
		f := db.File{
			FileObject: file,
			Referral:   referralId,
			ParentPath: parentPath,
		}
		_, ok := rh.Database.ServerCreateFile(f)
		if !ok {
			// TODO clean up
			lib.ErrorMessageHandler(w, r, 400, "Could not create file")
			return
		}
	}
	rh.Database.UpdatePayloadKeyById(referralId, response.PayloadKey)
	ok = rh.Database.UpdateStatusReferralById(referralId, db.UploadIncomplete)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not update referral")
		return
	}
	fmt.Println("Initiated: ", referralId)
	w.WriteHeader(201)
}

func (*UploadHandler) Error(w http.ResponseWriter, r *http.Request) {
}

// Situation: file tracking list exists filename:complete/incomplete,checksum
// This is per-upload tracking list
// addFileTracking = chunks for every file that want to upload check: file has to exist has to not be in tracking already (race condition), and cannot initiate for completed files
// updateTracking = new chunks are done, check file exists and chunkSeq exists in tracking
// updateFiles = new files are added, check filename same

func (rh *UploadHandler) AddFileTracking(files map[string]db.File, referralId int, newChunkTracking []ChunkFile) (err error) {
	if _, exists := rh.chunkTrackingList[referralId]; !exists {
		rh.chunkTrackingList[referralId] = make(map[string]ChunkFile, 0)
	}
	for _, cf := range newChunkTracking {
		// Check file exists
		referralFile, exists := files[cf.Name]
		if !exists {
			return fmt.Errorf("file '%s' does not exist in referral", cf.Name)
		}
		// Check if file is not done
		if referralFile.UploadStatus == db.CompleteUpload {
			return fmt.Errorf("file '%s' has already been uploaded", cf.Name)
		}
		// Check if file is in tracking already
		if _, exists := rh.chunkTrackingList[referralId][cf.Name]; exists {
			return fmt.Errorf("file '%s' is uploading", cf.Name)
		}
	}
	rh.chunkTrackingList[referralId] = make(map[string]ChunkFile)
	for _, cf := range newChunkTracking {
		// chunk status incomplete
		for i := range cf.Chunks {
			cf.Chunks[i].Status = Incomplete
		}
		rh.chunkTrackingList[referralId][cf.Name] = cf
	}
	return nil
}

func (rh UploadHandler) getIncompleteTrackingChunk(referralId int,
	filename string, chunkIndex int) (chunk Chunk, err error) {
	// referralId exists
	referralTracking, exists := rh.chunkTrackingList[referralId]
	if !exists {
		return Chunk{}, fmt.Errorf("referral '%d' is not accepting chunks", referralId)
	}
	// filename exists
	fileTracking, exists := referralTracking[filename]
	if !exists {
		return Chunk{}, fmt.Errorf("file '%s' is not accepting chunks", filename)
	}
	// chunkIndex is not out of bound
	if chunkIndex >= len(fileTracking.Chunks) {
		return Chunk{}, fmt.Errorf("chunk index is out of bounds")
	}
	// chunk alredy complete
	if fileTracking.Chunks[chunkIndex].Status == Complete {
		return Chunk{}, fmt.Errorf("chunk is already complete")
	}
	return fileTracking.Chunks[chunkIndex], nil
}

// Data Transfer
func (rh *UploadHandler) ChunkBegin(w http.ResponseWriter, r *http.Request) {
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	response := struct {
		ChunkFiles []ChunkFile `json:"ChunkFiles" validate:"unique=Name,required,dive"`
	}{}
	clientHospitalId := lib.GetContextHospital(r)
	// Syntax check
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
	if referral.Origin != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Origin mismatch: client does not have permission to upload")
		return
	}
	if referral.ReferralStatus != db.UploadIncomplete {
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Could not upload: referral is in state %s", referral.ReferralStatus))
		return
	}
	files, ok := rh.Database.GetFilesByReferral(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 400, "Could not get files for upload")
		return
	}
	fileMap := db.FilestoMap(files)
	err = rh.AddFileTracking(fileMap, referralId, response.ChunkFiles)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	fmt.Println("Chunk Begin: ", referralId)
	w.WriteHeader(201)
}

func (rh *UploadHandler) ChunkUpload(w http.ResponseWriter, r *http.Request) {
	clientHospitalId := lib.GetContextHospital(r)
	// URL vars
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
	chunkIndexString := mux.Vars(r)["chunkIndex"]
	chunkIndex, err := strconv.Atoi(chunkIndexString)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "could not parse chunkIndex")
		return
	}
	fmt.Println("Chunk Uploading: ", referralId, "Index:", chunkIndex)
	// Semantic Check
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Origin != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Origin mismatch: client does not have permission to upload")
		return
	}
	chunk, err := rh.getIncompleteTrackingChunk(referralId, filename, chunkIndex)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// Checksum
	var bodyBuffer bytes.Buffer
	hash := sha256.New()
	teeReader := io.TeeReader(r.Body, &bodyBuffer)
	if _, err := io.Copy(hash, teeReader); err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	resultChecksum := hex.EncodeToString(hash.Sum(nil))

	if resultChecksum != chunk.Checksum {
		lib.ErrorMessageHandler(w, r, 400, "Checksum mismatch")
		return
	}
	// Save data
	fo, err := lib.CreateFile(path.Join(
		rh.chunkDir,
		fmt.Sprintf("referral-%d", referralId),
		filename,
		fmt.Sprintf("chunk-%d", chunkIndex),
	))
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not save chunk")
		return
	}
	_, err = io.Copy(fo, &bodyBuffer)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not save chunk file")
		return
	}

	if err := fo.Close(); err != nil {
		lib.ErrorMessageHandler(w, r, 400, "Could not save chunk file")
		return
	}
	// chunk is saved
	rh.chunkTrackingList[referralId][filename].Chunks[chunkIndex].Status = Complete
	w.WriteHeader(200)
	fmt.Println("Chunk Done: ", referralId, "Index:", chunkIndex)
}

func MergeChunks(outPath string, inDir string, inFiles []string) (err error) {
	out, err := lib.CreateFile(outPath)
	if err != nil {
		return fmt.Errorf("could not open output file")
	}
	defer out.Close()
	for _, filename := range inFiles {
		filePath := path.Join(inDir, filename)
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("could not open chunk file")
		}
		_, err = io.Copy(out, file)
		if err != nil {
			return fmt.Errorf("could not append chunk to output")
		}
		file.Close()
	}
	return nil
}

func (rh *UploadHandler) Complete(w http.ResponseWriter, r *http.Request) {
	clientHospitalId := lib.GetContextHospital(r)
	// URL vars
	referralId, err := lib.GetReferralId(r)
	if err != nil {
		lib.ErrorMessageHandler(w, r, 400, err.Error())
		return
	}
	// // Semantic Check
	referral, ok := rh.Database.GetReferralById(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 404, "Could not find referral")
		return
	}
	if referral.Origin != clientHospitalId {
		lib.ErrorMessageHandler(w, r, 400, "Origin mismatch: client does not have permission to upload")
		return
	}
	// Work: Sync tracking with db files
	referralTracking, exists := rh.chunkTrackingList[referralId]
	if !exists {
		lib.ErrorMessageHandler(w, r, 400, fmt.Sprintf("Not tracking referral '%d'", referralId))
		return
	}
	files, ok := rh.Database.GetFilesByReferral(referralId)
	if !ok {
		lib.ErrorMessageHandler(w, r, 500, fmt.Sprintf("Could not find files for referral '%d'", referralId))
		return
	}
	fileMap := db.FilestoMap(files)

	// Check files complete, if so, update file
	for _, chunkfile := range referralTracking {
		fileComplete := true
		// for each file
		for _, chunk := range chunkfile.Chunks {
			if chunk.Status != Complete {
				fileComplete = false
				break // incomplete chunk, skip this file
			}
		}
		if fileComplete {
			// merge chunks
			outPath := path.Join(
				rh.payloadDir,
				fmt.Sprintf("referral-%d", referralId),
				fmt.Sprintf(chunkfile.Name),
			)
			inDir := path.Join(
				rh.chunkDir,
				fmt.Sprintf("referral-%d", referralId),
				fmt.Sprintf(chunkfile.Name),
			)
			inFiles := make([]string, 0)
			for i := range chunkfile.Chunks {
				inFiles = append(inFiles, fmt.Sprintf("chunk-%d", i))
			}
			err := MergeChunks(outPath, inDir, inFiles)
			if err != nil {
				lib.ErrorMessageHandler(w, r, 500, err.Error())
				return
			}
			// checksum
			hash := sha256.New()
			out, err := os.Open(outPath)
			if err != nil {
				lib.ErrorMessageHandler(w, r, 500, err.Error())
				return
			}
			if _, err := io.Copy(hash, out); err != nil {
				lib.ErrorMessageHandler(w, r, 400, err.Error())
				return
			}
			resultChecksum := hex.EncodeToString(hash.Sum(nil))
			targetFile := fileMap[chunkfile.Name]
			if resultChecksum != targetFile.Checksum {
				// Checksum error
				lib.ErrorMessageHandler(w, r, 400, "File checksum error")
				return
			}
			// update file state
			ok := rh.Database.UpdateStatusFileById(targetFile.Id, db.CompleteUpload)
			if !ok {
				lib.ErrorMessageHandler(w, r, 400, "File status update error")
				return
			}
			fmt.Println("Update complete", targetFile.Name)
		}

	}
	// Check all files complete
	allComplete := true
	files, _ = rh.Database.GetFilesByReferral(referralId)
	for _, file := range files {
		if file.UploadStatus != db.CompleteUpload {
			fmt.Println(file.UploadStatus)
			allComplete = false
			break
		}
	}
	if !allComplete {
		// TODO send tracking list as well
		lib.ErrorMessageHandler(w, r, 202, "Incomplete files")
		return
	}
	ok = rh.Database.UpdateStatusReferralById(referralId, db.UploadComplete)
	if !ok {
		lib.ErrorMessageHandler(w, r, 202, "Could not set referral")
		return
	}
	w.WriteHeader(200)
}
