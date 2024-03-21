package client

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	lib "simplemts/lib"
)

type Client struct {
	service *http.Client
}

func NewClient(cert string, key string, ca_crt string) (client Client) {
	certificate, err := lib.LoadCert(cert, key)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool, err := lib.LoadPool(ca_crt)
	if err != nil {
		log.Fatal(err)
	}
	client = Client{
		service: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:      caCertPool,
					Certificates: []tls.Certificate{certificate},
				},
			},
		},
	}
	return
}
func (c *Client) MakeJsonRequestRaw(URL string, body string) (io.ReadCloser, int, error) {
	// Create
	bodyReader := bytes.NewReader([]byte(body))
	req, err := http.NewRequest(http.MethodPost, URL, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		return nil, 500, err
	}

	// Making Request
	res, err := c.service.Do(req)
	if err != nil {
		return nil, 500, err
	}
	return res.Body, res.StatusCode, nil
}

func (c *Client) MakeJsonRequest(URL string, body string) (string, int, error) {
	bodyCloser, code, err := c.MakeJsonRequestRaw(URL, body)
	if err != nil {
		return "", code, err
	}

	// Parse
	defer bodyCloser.Close()
	bodyBytes, err := io.ReadAll(bodyCloser)
	if err != nil {
		return "", 500, err
	}
	return string(bodyBytes), code, nil
}
func (c *Client) MakeGetRequestRaw(URL string) (io.ReadCloser, int, error) {
	// Create
	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return nil, 500, err
	}

	// Making Request
	res, err := c.service.Do(req)
	if err != nil {
		return nil, 500, err
	}
	return res.Body, res.StatusCode, nil
}

func (c *Client) MakeGetRequest(URL string) (string, int, error) {
	bodyCloser, code, err := c.MakeGetRequestRaw(URL)
	if err != nil {
		return "", code, err
	}
	// Parse
	defer bodyCloser.Close()
	bodyBytes, err := io.ReadAll(bodyCloser)
	if err != nil {
		return "", 500, err
	}
	return string(bodyBytes), code, nil
}

func (c *Client) MakePostBinaryRaw(URL string, bodyReader io.Reader) (io.ReadCloser, int, error) {
	// Create
	req, err := http.NewRequest(http.MethodPost, URL, bodyReader)
	req.Header.Set("Content-Type", "application/octet-stream")
	if err != nil {
		return nil, 500, err
	}

	// Making Request
	res, err := c.service.Do(req)
	if err != nil {
		return nil, 500, err
	}
	return res.Body, res.StatusCode, nil
}

func (c *Client) MakePostBinary(URL string, bodyReader io.Reader) (string, int, error) {
	bodyCloser, code, err := c.MakePostBinaryRaw(URL, bodyReader)
	if err != nil {
		return "", code, err
	}
	// Parse
	defer bodyCloser.Close()
	bodyBytes, err := io.ReadAll(bodyCloser)
	if err != nil {
		return "", 500, err
	}
	return string(bodyBytes), code, nil
}
