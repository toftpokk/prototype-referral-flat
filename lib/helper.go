package lib

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

type Requester interface {
	MakeJsonRequest(URL string, body string) (string, int, error)
	MakeJsonRequestRaw(URL string, body string) (io.ReadCloser, int, error)
	MakeGetRequest(URL string) (string, int, error)
	MakeGetRequestRaw(URL string) (io.ReadCloser, int, error)
	MakePostBinary(URL string, bodyReader io.Reader) (string, int, error)
	MakePostBinaryRaw(URL string, bodyReader io.Reader) (io.ReadCloser, int, error)
}

func GetEnv(key string, defaultVal string) string {
	value, found := os.LookupEnv(key)
	if !found {
		return defaultVal
	}
	return value
}

func GetEnvAsInt(key string, defaultVal int) int {
	valueStr := GetEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

func LoadCert(own_crt string, own_key string) (cert tls.Certificate, err error) {
	cert, err = tls.LoadX509KeyPair(own_crt, own_key)
	if err != nil {
		return
	}
	return
}

func LoadPool(ca_crt string) (caCertPool *x509.CertPool, err error) {
	caCert, err := os.ReadFile(ca_crt)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool = x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	return
}

func DecodeValidate(response any, reader io.ReadCloser) error {
	err := json.NewDecoder(reader).Decode(response)
	if err != nil {
		return fmt.Errorf("could not decode data: %s", err.Error())
	}
	validate := validator.New()
	err = validate.Struct(response)
	if err != nil {
		return fmt.Errorf("validation error: %s", err)
	}
	return nil
}

func ErrorMessageHandler(w http.ResponseWriter, r *http.Request, status int, message string) {
	// TODO no 'Error' handler, but any thing with JSON
	w.WriteHeader(status)
	errs := struct {
		Message string `json:"message"`
	}{
		Message: message,
	}
	jsonString, _ := json.Marshal(errs)
	fmt.Fprint(w, string(jsonString))
}

func IfFatalError(err error, code int, message []byte) {
	if err != nil {
		log.Fatal(err)
	}
	if code >= 300 || code < 200 {
		log.Fatalf("Server error: %d %s", code, message)
	}
}

// Context Keys
type ContextKey int

const (
	KeyHospital ContextKey = iota
)

func GetContextHospital(r *http.Request) string {
	return r.Context().Value(KeyHospital).(string)
}

func AddHospitalContext(request *http.Request, clientId string) *http.Request {
	return request.WithContext(context.WithValue(request.Context(), KeyHospital, clientId))
}

func GetReferralId(r *http.Request) (referralId int, err error) {
	referralIdString := mux.Vars(r)["referralId"]
	referralId, err = strconv.Atoi(referralIdString)
	if err != nil {
		return 0, fmt.Errorf("could not parse referralId")
	}
	return referralId, nil
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")

		if r.Method == "OPTIONS" {
			http.Error(w, "No Content", http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func CreateFile(p string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(p), 0770); err != nil {
		return nil, err
	}
	return os.Create(p)
}
