package frontendhandler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	testhelper "simplemts/lib/testHelper"
	frontendhandler "simplemts/referralClient/frontendHandler"
	"testing"
)

// var testClient = client.NewClient()
var handler = frontendhandler.RouteHander{
	ServerURL: "MOCK_SERVER_URL",
}

type MockRequester = testhelper.MockRequester

func TestCreate(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		// Mock Server
		requester := MockRequester{
			ResponseStatus: 201,
			ResponseData:   []byte(`{"id":"1234"}`),
		}
		handler.Client = &requester
		//
		bodyReader := bytes.NewReader([]byte(`{
			"Origin": "12345",
			"Destination": "67890",
			"Department": "a",
			"Reason": "a",
			"CitizenId": "b",
			"Prefix": "mr",
			"FirstName": "b",
			"LastName": "b",
			"BirthDate": "2006-02-01",
			"Address": "b",
			"Gender": "male",
			"Telephone": "0000000000",
			"Email": "a@b.c",
			"Diagnosis": "a",
			"History" : "a"
		}`))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		response := httptest.NewRecorder()
		handler.CreateReferral(response, request)

		// Results
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 201
		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"id":"[^"]+"}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
}
func TestListReferral(t *testing.T) {
	t.Run("Normal", func(t *testing.T) {
		// Mock Server
		requester := MockRequester{
			ResponseStatus: 200,
			ResponseData:   []byte(`MOCK_DATA`),
		}
		handler.Client = &requester
		//
		request, _ := http.NewRequest(http.MethodPost, "/outgoing", nil)
		response := httptest.NewRecorder()
		handler.ListReferralDoctor(response, request)

		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200
		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}
