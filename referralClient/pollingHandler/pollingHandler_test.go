package pollinghandler_test

import (
	"fmt"
	db "simplemts/lib/database"
	testhelper "simplemts/lib/testHelper"
	pollinghandler "simplemts/referralClient/pollingHandler"
	"testing"
)

var database = db.NewDatabase("../../testing_client.sqlite")
var mockRequester = testhelper.MockRequester{}
var handler = pollinghandler.NewPollingHandler(
	1, &mockRequester, &database, "SERVER_URL")

func TestHandleIncoming(t *testing.T) {
	t.Run("Consented", func(t *testing.T) {
		pollData := pollinghandler.PollData{
			Id:             12345,
			ReferralStatus: db.Consented,
		}
		mockRequester.ResponseStatus = 200
		mockRequester.ResponseData = []byte("a")
		handler.HandleIncoming(pollData)
		// TODO test email
	})
}

// Download handler
func TestHandleDownload(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		referralId := 12345
		mockRequester.ResponseStatus = 200
		mockRequester.ResponseData = []byte(`{
			"Files" : [
				{
					"Name": "a",
					"UploadStatus": "UploadComplete",
					"Checksum": "UploadComplete"
				}
			]
		}`)
		response, _, err := handler.DownloadList(referralId)
		wantUrl := fmt.Sprintf("SERVER_URL/%d/download", referralId)
		gotUrl := mockRequester.RequestURL
		if gotUrl != wantUrl {
			t.Errorf("Want %s, Got %s", wantUrl, gotUrl)
		}
		if err != nil {
			t.Errorf("Error %s", err)
		}
		fmt.Println(response)
		want := []pollinghandler.FileTracking{
			{
				Name:         "a",
				UploadStatus: "UploadComplete",
				Checksum:     "UploadComplete",
			},
		}
		fmt.Println(response)
		if response[0] != want[0] {
			t.Errorf("Wrong result: %s", response)
		}
	})
}

func TestChunkBegin(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
	})
}
