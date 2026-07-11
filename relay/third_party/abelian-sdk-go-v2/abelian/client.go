package abelian

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DEFAULT_REQUEST_TIMEOUT = 3000
)

type ClientConfig struct {
	Endpoint string

	EnableTLS bool   // enable tls
	CaFile    string // cert file for verifying server certificates

	Username string // username for auth
	Password string // password for auth

	Timeout uint64 // timeout for requesting Abec server, default DEFAULT_TIMEOUT
}

func NewClientConfig(endpoint string, options ...ClientOption) *ClientConfig {
	clientConfig := &ClientConfig{
		Endpoint: endpoint,
		Timeout:  DEFAULT_REQUEST_TIMEOUT,
	}

	for _, opt := range options {
		opt(clientConfig)
	}

	return clientConfig
}

// ClientOption change client config
type ClientOption func(*ClientConfig)

// WithTimeout ...
func WithTimeout(timeout uint64) ClientOption {
	return func(config *ClientConfig) {
		config.Timeout = timeout
	}
}

// WithAuth ...
func WithAuth(username string, password string) ClientOption {
	return func(config *ClientConfig) {
		config.Username = username
		config.Password = password
	}
}

func WithTLS(caFile string) ClientOption {
	return func(config *ClientConfig) {
		config.EnableTLS = true
		config.CaFile = caFile
	}
}

type Client struct {
	*http.Client
	Endpoint string
	Username string // username for basic auth
	Password string // password for basic auth

}

func NewClient(config *ClientConfig) (*Client, error) {
	httpClient := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}
	if config.EnableTLS {
		caCert, err := os.ReadFile(config.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to add CA certificate to pool")
		}

		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				ServerName: config.Endpoint,
			},
		}
	}
	return &Client{
		Client:   httpClient,
		Endpoint: config.Endpoint,
		Username: config.Username,
		Password: config.Password,
	}, nil
}

func (client *Client) Do(method string, params []interface{}, result any) error {
	jsonReq := &JSONRPCRequest{
		JSONRPC: "1.0",
		Method:  method,
		Params:  params,
		ID:      strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	jsonBody, err := json.Marshal(jsonReq)
	if err != nil {
		sdkLog.Errorf("fail to marshal json request: %v", err)
		return err
	}

	req, err := http.NewRequest(http.MethodPost, client.Endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		sdkLog.Errorf("fail to create request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(client.Username, client.Password)

	resp, err := client.Client.Do(req)
	if err != nil {
		sdkLog.Errorf("fail to do request: %v", err)
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	respObj := &JSONRPCResponse{}
	err = json.Unmarshal(body, respObj)
	if err != nil {
		sdkLog.Errorf("fail to unmarshal json response: %v", err)
		return err
	}
	if respObj.Error != nil {
		sdkLog.Errorf("request method %s with param %v, response error: %v", method, params, respObj.Error)
		return respObj.Error
	}

	if result == nil {
		return nil
	}

	err = json.Unmarshal(respObj.Result, &result)
	if err != nil {
		sdkLog.Errorf("fail to unmarshal json result: %v", err)
		return err
	}
	return nil
}
