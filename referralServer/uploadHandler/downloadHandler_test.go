package uploadhandler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"simplemts/lib"
	db "simplemts/lib/database"
	testhelper "simplemts/lib/testHelper"
	uploadhandler "simplemts/referralServer/uploadHandler"
	"testing"

	"github.com/gorilla/mux"
)

func createCompleted(t *testing.T, clientHospitalId string, uploadHandler uploadhandler.UploadHandler) (referralId int) {
	referralId, _ = testhelper.CreateMockReferral(uploadHandler.Database)
	t.Logf("Created Referral %d\n", referralId)
	uploadHandler.Database.UpdateStatusReferralById(referralId, db.UploadIncomplete)
	parentPath := path.Join("", fmt.Sprint(referralId))

	files := testhelper.CreateMockChunkBegin(&database, referralId, parentPath, []db.FileObject{
		{
			Name:     "a",
			Checksum: "0c6f8086798a901a754243bb8e90ec638a93de00b2b272484d003d8f0f582472",
		},
	}, uploadHandler, map[string][]uploadhandler.Chunk{
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
	chunkfile := db.ChunkFile{
		Name: "a",
		Chunks: []db.Chunk{
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
	}
	uploadChunks(referralId, "a", 3, clientHospitalId, uploadHandler)
	// outPath := path.Join(
	// 	uploadHandler.Path,
	// 	fmt.Sprintf("referral-%d", referralId),
	// 	fmt.Sprintf(chunkfile.Name),
	// )
	// inDir := path.Join(
	// 	uploadHandler.ChunkPath,
	// 	fmt.Sprintf("referral-%d", referralId),
	// 	fmt.Sprintf(chunkfile.Name),
	// )
	inFiles := make([]string, 0)
	for i := range chunkfile.Chunks {
		inFiles = append(inFiles, fmt.Sprintf("chunk-%d", i))
	}
	uploadhandler.MergeChunks("", "", inFiles)
	uploadHandler.Database.UpdateStatusReferralById(referralId, db.UploadComplete)
	uploadHandler.Database.UpdateStatusFileById(files[0].Id, db.CompleteUpload)
	return
}

func TestGetFiles(t *testing.T) {
	clientHospitalId := destinationHospitalId
	referralId := createCompleted(t, originHospitalId, handler)

	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/upload", nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.GetFiles(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"Files":\[{"UploadStatus":"UploadComplete","Name":"a","Checksum":".*"}\]}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
}
func TestDownloadFile(t *testing.T) {
	clientHospitalId := destinationHospitalId
	referralId := createCompleted(t, originHospitalId, handler)

	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/{referralId}/download/{filename}", nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
			"filename":   "a",
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.DownloadFile(response, requestWithVars)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}
