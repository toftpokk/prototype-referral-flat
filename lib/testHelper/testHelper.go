package testing

import (
	"encoding/json"
	"fmt"
	"io"
	db "simplemts/lib/database"
	uploadhandler "simplemts/referralServer/uploadHandler"
	"strings"
)

type Creation struct {
	db.PatientObject
	db.ReferralObject
}

func GenerateMockCreation(c Creation) (output []byte) {
	if c.Origin == "" {
		c.Origin = "12345"
	}
	if c.Destination == "" {
		c.Destination = "67890"
	}
	if c.Department == "" {
		c.Department = "a"
	}
	if c.Reason == "" {
		c.Reason = "b"
	}

	if c.CitizenId == "" {
		c.CitizenId = "c"
	}
	if c.Prefix == "" {
		c.Prefix = "mr"
	}
	if c.FirstName == "" {
		c.FirstName = "d"
	}
	if c.LastName == "" {
		c.LastName = "e"
	}
	if c.BirthDate == "" {
		c.BirthDate = "2006-02-01"
	}
	if c.Address == "" {
		c.Address = "f"
	}
	if c.Gender == "" {
		c.Gender = "male"
	}
	if c.Telephone == "" {
		c.Telephone = "0000000000"
	}
	if c.Email == "" {
		c.Email = "b@a.b"
	}
	output, _ = json.Marshal(c)
	return
}

func CreateMockReferral(database *db.Database) (id int, ok bool) {
	return database.CreateReferralServer(db.Referral{
		ReferralObject: db.ReferralObject{
			Origin:      "12345",
			Destination: "67890",
			Department:  "a",
			Reason:      "a",
		},
		PatientObject: db.PatientObject{
			CitizenId: "b",
			Prefix:    "mr",
			FirstName: "b",
			LastName:  "b",
			BirthDate: "0000-00-00",
			Address:   "b",
			Gender:    "male",
			Telephone: "0000000000",
			Email:     "b",
		},
	})

}

func CreateMockFiles(database *db.Database, referralId int, parentPath string, fileObjects []db.FileObject) {
	for _, fo := range fileObjects {
		database.ServerCreateFile(db.File{
			Referral:   referralId,
			ParentPath: parentPath,
			FileObject: fo,
		})
	}
}

func CreateMockChunkBegin(
	database *db.Database, referralId int, parentPath string,
	fileObjects []db.FileObject, uploadHandler uploadhandler.UploadHandler,
	chunks map[string]([]uploadhandler.Chunk),
) (files []db.File) {
	CreateMockFiles(database, referralId, parentPath, fileObjects)
	files, _ = database.GetFilesByReferral(referralId)
	fileMap := db.FilestoMap(files)
	newChunkTracking := []uploadhandler.ChunkFile{}
	for _, file := range files {
		c, exists := chunks[file.Name]
		if !exists {
			fmt.Println("Chunk name error")
			return
		}
		newChunkTracking = append(newChunkTracking, uploadhandler.ChunkFile{
			Name:   file.Name,
			Chunks: c,
		})
	}
	uploadHandler.AddFileTracking(fileMap, referralId, newChunkTracking)
	return
}

type MockRequester struct {
	ResponseStatus    int
	ResponseData      []byte
	RequestURL        string
	RequestBody       string
	RequestBodyReader io.Reader
}

func (mr *MockRequester) MakeJsonRequestRaw(URL string, body string) (io.ReadCloser, int, error) {
	mr.RequestURL = URL
	mr.RequestBody = body
	return io.NopCloser(strings.NewReader(string(mr.ResponseData))), mr.ResponseStatus, nil
}

func (mr *MockRequester) MakeGetRequestRaw(URL string) (io.ReadCloser, int, error) {
	mr.RequestURL = URL
	mr.RequestBody = ""
	return io.NopCloser(strings.NewReader(string(mr.ResponseData))), mr.ResponseStatus, nil
}

func (mr *MockRequester) MakeJsonRequest(URL string, body string) (string, int, error) {
	mr.RequestURL = URL
	mr.RequestBody = body
	return string(mr.ResponseData), mr.ResponseStatus, nil
}

func (mr *MockRequester) MakeGetRequest(URL string) (string, int, error) {
	mr.RequestURL = URL
	mr.RequestBody = ""
	return string(mr.ResponseData), mr.ResponseStatus, nil
}

func (mr *MockRequester) MakePostBinary(URL string, bodyReader io.Reader) (string, int, error) {
	mr.RequestURL = URL
	mr.RequestBodyReader = bodyReader
	return string(mr.ResponseData), mr.ResponseStatus, nil
}
func (mr *MockRequester) MakePostBinaryRaw(URL string, bodyReader io.Reader) (io.ReadCloser, int, error) {
	mr.RequestURL = URL
	mr.RequestBodyReader = bodyReader
	return io.NopCloser(strings.NewReader(string(mr.ResponseData))), mr.ResponseStatus, nil
}
