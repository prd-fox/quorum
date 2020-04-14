package tessera

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	testifyassert "github.com/stretchr/testify/assert"

	"github.com/ethereum/go-ethereum/private/engine"
)

var (
	arbitraryHash           = []byte("arbitrary")
	arbitraryHash1          = []byte("arbitrary1")
	arbitraryNotFoundHash   = []byte("not found")
	arbitraryPrivatePayload = []byte("arbitrary private payload")
	arbitraryFrom           = "arbitraryFrom"
	arbitraryTo             = []string{"arbitraryTo1", "arbitraryTo2"}

	testServer *httptest.Server
	testObject *tesseraPrivateTxManager

	sendRequestCaptor         = make(chan *capturedRequest)
	receiveRequestCaptor      = make(chan *capturedRequest)
	sendSignedTxRequestCaptor = make(chan *capturedRequest)
)

type capturedRequest struct {
	err     error
	request interface{}
	header  http.Header
}

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	teardown()
	os.Exit(retCode)
}

func Must(o interface{}, err error) interface{} {
	if err != nil {
		panic(fmt.Sprintf("%s", err))
	}
	return o
}

func setup() {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", MockSendAPIHandlerFunc)
	mux.HandleFunc("/transaction/", MockReceiveAPIHandlerFunc)
	mux.HandleFunc("/sendsignedtx", MockSendSignedTxAPIHandlerFunc)

	testServer = httptest.NewServer(mux)

	testObject = New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})
}

func MockSendAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	actualRequest := new(sendRequest)
	if err := json.NewDecoder(request.Body).Decode(actualRequest); err != nil {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{err: err})
	} else {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{request: actualRequest, header: request.Header})
		data, _ := json.Marshal(&sendResponse{
			Key: base64.StdEncoding.EncodeToString(arbitraryHash),
		})
		response.Write(data)
	}
}

func MockReceiveAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	path := string([]byte(request.RequestURI)[:strings.LastIndex(request.RequestURI, "?")])
	actualRequest, err := url.PathUnescape(strings.TrimPrefix(path, "/transaction/"))
	if err != nil {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{err: err})
	} else {
		go func(o *capturedRequest) {
			receiveRequestCaptor <- o
		}(&capturedRequest{request: actualRequest, header: request.Header})
		if actualRequest == base64.StdEncoding.EncodeToString(arbitraryNotFoundHash) {
			response.WriteHeader(http.StatusNotFound)
		} else {
			data, _ := json.Marshal(&receiveResponse{
				Payload: base64.StdEncoding.EncodeToString(arbitraryPrivatePayload),
			})
			response.Write(data)
		}
	}
}

func MockSendSignedTxAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	actualRequest := new(sendSignedTxRequest)
	if err := json.NewDecoder(request.Body).Decode(actualRequest); err != nil {
		go func(o *capturedRequest) { sendSignedTxRequestCaptor <- o }(&capturedRequest{err: err})
	} else {
		go func(o *capturedRequest) { sendSignedTxRequestCaptor <- o }(&capturedRequest{request: actualRequest, header: request.Header})
		data, _ := json.Marshal(&sendSignedTxResponse{
			Key: base64.StdEncoding.EncodeToString(arbitraryHash),
		})
		response.Write(data)
	}
}

func teardown() {
	testServer.Close()
}

func verifyRequetHeader(h http.Header, t *testing.T) {
	if h.Get("Content-type") != "application/json" {
		t.Errorf("expected Content-type header is application/json")
	}

	if h.Get("Accept") != "application/json" {
		t.Errorf("expected Accept header is application/json")
	}
}

func TestSend_whenTypical(t *testing.T) {
	assert := testifyassert.New(t)

	actualHash, err := testObject.Send(arbitraryPrivatePayload, arbitraryFrom, arbitraryTo)
	if err != nil {
		t.Fatalf("%s", err)
	}
	capturedRequest := <-sendRequestCaptor

	if capturedRequest.err != nil {
		t.Fatalf("%s", capturedRequest.err)
	}

	verifyRequetHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(*sendRequest)

	assert.Equal(arbitraryPrivatePayload, actualRequest.Payload, "request.payload")
	assert.Equal(arbitraryFrom, actualRequest.From, "request.from")
	assert.Equal(arbitraryTo, actualRequest.To, "request.to")
	assert.Equal(arbitraryHash, actualHash, "returned hash")
}

func TestReceive_whenTypical(t *testing.T) {
	assert := testifyassert.New(t)

	_, err := testObject.Receive(arbitraryHash1)
	if err != nil {
		t.Fatalf("%s", err)
	}
	capturedRequest := <-receiveRequestCaptor

	if capturedRequest.err != nil {
		t.Fatalf("%s", capturedRequest.err)
	}

	verifyRequetHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(string)

	assert.Equal(base64.StdEncoding.EncodeToString(arbitraryHash1), actualRequest, "requested hash")
}

func TestReceive_whenPayloadNotFound(t *testing.T) {
	assert := testifyassert.New(t)

	data, err := testObject.Receive(arbitraryNotFoundHash)
	if err != nil {
		t.Fatalf("%s", err)
	}
	capturedRequest := <-receiveRequestCaptor

	if capturedRequest.err != nil {
		t.Fatalf("%s", capturedRequest.err)
	}

	verifyRequetHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(string)

	assert.Equal(base64.StdEncoding.EncodeToString(arbitraryNotFoundHash), actualRequest, "requested hash")
	assert.Nil(data, "returned payload when not found")
}

func TestSendSignedTx_whenTypical(t *testing.T) {
	assert := testifyassert.New(t)

	_, err := testObject.SendSignedTx(arbitraryHash, arbitraryTo)
	if err != nil {
		t.Fatalf("%s", err)
	}
	capturedRequest := <-sendSignedTxRequestCaptor

	if capturedRequest.err != nil {
		t.Fatalf("%s", capturedRequest.err)
	}

	verifyRequetHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(*sendSignedTxRequest)

	assert.Equal(arbitraryTo, actualRequest.To, "request.to")
}
