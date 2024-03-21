package routehandler_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"simplemts/lib"
	db "simplemts/lib/database"
	testhelper "simplemts/lib/testHelper"
	routehandler "simplemts/referralServer/routeHandler"
	"testing"

	"github.com/gorilla/mux"
)

var database = db.NewDatabase("../../testing.sqlite")
var handler = routehandler.RouteHander{
	Database: &database,
}

type Creation = testhelper.Creation

const destinationHospitalId = "67890"
const originHospitalId = "12345"

func TestCreate(t *testing.T) {
	clientHospitalId := originHospitalId
	t.Run("Normal", func(t *testing.T) {
		bodyReader := bytes.NewReader(testhelper.GenerateMockCreation(Creation{}))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.CreateReferral(response, requestWithContext)
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
	t.Run("Wrong Prefix", func(t *testing.T) {
		bodyReader := bytes.NewReader(testhelper.GenerateMockCreation(Creation{PatientObject: db.PatientObject{Prefix: "m"}}))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.CreateReferral(response, requestWithContext)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 400

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"message":"prefix should be mr, mrs, ms"}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
	t.Run("Wrong BirthDate", func(t *testing.T) {
		bodyReader := bytes.NewReader(testhelper.GenerateMockCreation(Creation{PatientObject: db.PatientObject{BirthDate: "m"}}))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.CreateReferral(response, requestWithContext)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 400

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"message":"validation error: Key: 'PatientObject.BirthDate' Error:Field validation for 'BirthDate' failed on the 'datetime' tag"}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
	t.Run("Wrong Gender", func(t *testing.T) {
		bodyReader := bytes.NewReader(testhelper.GenerateMockCreation(Creation{PatientObject: db.PatientObject{Gender: "m"}}))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.CreateReferral(response, requestWithContext)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 400

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"message":"gender should be male or female"}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
	t.Run("Wrong Telephone", func(t *testing.T) {
		bodyReader := bytes.NewReader(testhelper.GenerateMockCreation(Creation{PatientObject: db.PatientObject{Telephone: "m"}}))
		request, _ := http.NewRequest(http.MethodPost, "/", bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.CreateReferral(response, requestWithContext)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 400

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"message":"telephone should have 10 digits"}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
}

func TestPollIncoming(t *testing.T) {
	clientHospitalId := destinationHospitalId
	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.Poll(response, requestWithContext, false)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"referrals":\[({"Id":[^,]*,"ReferralStatus":"[^"]*"},?)*\]}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
}

func TestPollOutgoing(t *testing.T) {
	clientHospitalId := destinationHospitalId
	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		response := httptest.NewRecorder()

		handler.Poll(response, requestWithContext, true)
		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
		match, _ := regexp.MatchString(`^{"referrals":\[({"Id":[^,]*,"ReferralStatus":"[^"]*"},?)*\]}$`, got)
		if !match {
			t.Errorf(`Unexpected response: "%s"`, got)
			return
		}
	})
}

func TestGrant(t *testing.T) {
	clientHospitalId := destinationHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.Consented)
	t.Run("Normal Grant", func(t *testing.T) {
		bodyReader := bytes.NewReader([]byte(`{
			"Granted": true
		}`))
		request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/%d/grant", referralId), bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		// Vars
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.GrantReferral(response, requestWithVars)

		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
	referralId, _ = testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.Consented)
	t.Run("Normal Not Grant", func(t *testing.T) {
		bodyReader := bytes.NewReader([]byte(`{
			"Granted": false
		}`))
		request, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/%d/grant", referralId), bodyReader)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		// Vars
		vars := map[string]string{
			"referralId": fmt.Sprint(referralId),
		}
		requestWithVars := mux.SetURLVars(requestWithContext, vars)
		response := httptest.NewRecorder()
		handler.GrantReferral(response, requestWithVars)

		gotstatus := response.Result().StatusCode
		got := response.Body.String()
		wantstatus := 200

		if gotstatus != wantstatus {
			t.Errorf("got %d, want %d: response: %s\b", gotstatus, wantstatus, got)
			return
		}
	})
}

func TestComplete(t *testing.T) {
	clientHospitalId := destinationHospitalId
	referralId, _ := testhelper.CreateMockReferral(handler.Database)
	t.Logf("Created Referral %d\n", referralId)
	handler.Database.UpdateStatusReferralById(referralId, db.UploadComplete)
	t.Run("Normal", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/%d/complete", referralId), nil)
		requestWithContext := lib.AddHospitalContext(request, clientHospitalId)
		// Vars
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
