package uploadhandler_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"simplemts/lib"
	db "simplemts/lib/database"
	testhelper "simplemts/lib/testHelper"
	uploadhandler "simplemts/referralServer/uploadHandler"
	"testing"

	"github.com/gorilla/mux"
)

var database = db.NewDatabase("../../testing.sqlite")
var handler = uploadhandler.NewUploadHandler(&database)

const destinationHospitalId = "67890"
const originHospitalId = "12345"

func TestInitiate(t *testing.T) {
	clientHospitalId := originHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.Granted)

	t.Run("Normal", func(t *testing.T) {
		bodyReader := bytes.NewReader([]byte(`{
			"Files": [
				{"Name":"a","Checksum":"OK"},
				{"Name":"b","Checksum":"OK"},
				{"Name":"c","Checksum":"OK"}
			]
		}`))
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.Initiate(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 201

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}

func TestChunkBegin(t *testing.T) {
	clientHospitalId := originHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.UploadIncomplete)
	parentPath := path.Join("", fmt.Sprint(referralId))
	// Mock files
	testhelper.CreateMockFiles(handler.Database, referralId, parentPath, []db.FileObject{
		{
			Name:     "a",
			Checksum: "b",
		},
		{
			Name:     "b",
			Checksum: "b",
		},
		{
			Name:     "c",
			Checksum: "b",
		},
	})

	t.Run("Normal", func(t *testing.T) {
		bodyReader := bytes.NewReader([]byte(`{
			"ChunkFiles": [
				{
					"Name" : "a",
					"Chunks":[
					]
				},
				{
					"Name" : "b",
					"Chunks":[
					]
				}
			]
		}`))
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.ChunkBegin(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 201

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}

func TestChunkUpload(t *testing.T) {
	clientHospitalId := originHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.UploadIncomplete)
	parentPath := path.Join("", fmt.Sprint(referralId))

	testhelper.CreateMockChunkBegin(&database, referralId, parentPath, []db.FileObject{
		{
			Name:     "a",
			Checksum: "a",
		},
		{
			Name:     "b",
			Checksum: "b",
		},
		{
			Name:     "c",
			Checksum: "c",
		},
	}, handler, map[string][]uploadhandler.Chunk{
		"a": {
			{
				Checksum: "0a287497614e7d7560f9c43fcb3e1a7d80eb7733402d6209694600784e08fa44",
				SizeKB:   1,
			},
		},
		"b": {
			{
				Checksum: "0a287497614e7d7560f9c43fcb3e1a7d80eb7733402d6209694600784e08fa44",
				SizeKB:   1,
			},
		},
		"c": {
			{
				Checksum: "0a287497614e7d7560f9c43fcb3e1a7d80eb7733402d6209694600784e08fa44",
				SizeKB:   1,
			},
		},
	})
	t.Run("Normal", func(t *testing.T) {
		f, _ := os.Open("../../test/sample.txt")
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", f)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
			"filename":   "a",
			"chunkIndex": "0",
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.ChunkUpload(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
	t.Run("Normal (2nd)", func(t *testing.T) {
		f, _ := os.Open("../../test/sample.txt")
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", f)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
			"filename":   "b",
			"chunkIndex": "0",
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.ChunkUpload(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}

func uploadChunks(
	referralId int, filename string,
	chunkCount int, clientHospitalId string,
	uploadHandler uploadhandler.UploadHandler,
) {
	for i := 0; i < chunkCount; i++ {
		f, _ := os.Open(fmt.Sprintf("../../test/chunk-%d", i))
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", f)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
			"filename":   filename,
			"chunkIndex": fmt.Sprint(i),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		uploadHandler.ChunkUpload(response, requestWithVars)
		if response.Result().StatusCode != 200 {
			fmt.Printf("Error %s", response.Body.String())
		}
	}
}

func TestComplete(t *testing.T) {
	clientHospitalId := originHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.UploadIncomplete)
	parentPath := path.Join("", fmt.Sprint(referralId))

	testhelper.CreateMockChunkBegin(&database, referralId, parentPath, []db.FileObject{
		{
			Name:     "a",
			Checksum: "0c6f8086798a901a754243bb8e90ec638a93de00b2b272484d003d8f0f582472",
		},
	}, handler, map[string][]uploadhandler.Chunk{
		"a": {
			{
				Checksum: "92a70d3be38622f24529577b15986214465e4a8c635b7a6f5244944f36a55327",
				SizeKB:   1,
			},
			{
				Checksum: "bebb51c60cb98cb22531fc3d17011a8ba7f091fcd3d837140988877a8288a019",
				SizeKB:   1,
			},
			{
				Checksum: "061cf22ea81af8c97c2a666373b5a4bbb798c45f91b5a60259b8747904e86ea7",
				SizeKB:   1,
			},
		},
	})
	uploadChunks(referralId, "a", 3, clientHospitalId, handler)
	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.Complete(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}
