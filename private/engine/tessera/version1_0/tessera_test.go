package version1_0

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/private/engine"
	"github.com/stretchr/testify/assert"
)

var (
	emptyHash                      = common.EncryptedPayloadHash{}
	arbitraryHash                  = common.BytesToEncryptedPayloadHash([]byte("arbitrary"))
	arbitraryHash1                 = common.BytesToEncryptedPayloadHash([]byte("arbitrary1"))
	arbitraryNotFoundHash          = common.BytesToEncryptedPayloadHash([]byte("not found"))
	arbitraryHashNoPrivateMetadata = common.BytesToEncryptedPayloadHash([]byte("no private extra data"))
	arbitraryPrivatePayload        = []byte("arbitrary private payload")
	arbitraryFrom                  = "arbitraryFrom"
	arbitraryTo                    = []string{"arbitraryTo1", "arbitraryTo2"}
	arbitraryPrivacyFlag           = engine.PrivacyFlagPartyProtection
	standardPrivateExtra           = &engine.ExtraMetadata{
		PrivacyFlag: engine.PrivacyFlagStandardPrivate,
	}

	testServer *httptest.Server
	testObject *tesseraPrivateTxManager

	sendRequestCaptor                    = make(chan *capturedRequest)
	receiveRequestCaptor                 = make(chan *capturedRequest)
	sendSignedTxOctetStreamRequestCaptor = make(chan *capturedRequest)
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

func setup() {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", MockSendAPIHandlerFunc)
	mux.HandleFunc("/transaction/", MockReceiveAPIHandlerFunc)
	mux.HandleFunc("/sendsignedtx", MockSendSignedTxOctetStreamAPIHandlerFunc)
	mux.HandleFunc("/storeraw", MockStoreRawAPIHandlerFunc)

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
			Key: arbitraryHash.ToBase64(),
		})
		response.Write(data)
	}
}

func MockStoreRawAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	actualRequest := new(storerawRequest)
	if err := json.NewDecoder(request.Body).Decode(actualRequest); err != nil {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{err: err})
	} else {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{request: actualRequest, header: request.Header})
		data, _ := json.Marshal(&sendResponse{
			Key: arbitraryHash.ToBase64(),
		})
		response.Write(data)
	}
}

func MockReceiveAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	actualRequest, err := url.PathUnescape(strings.TrimPrefix(request.RequestURI, "/transaction/"))
	if err != nil {
		go func(o *capturedRequest) { sendRequestCaptor <- o }(&capturedRequest{err: err})
	} else {
		go func(o *capturedRequest) {
			receiveRequestCaptor <- o
		}(&capturedRequest{request: actualRequest, header: request.Header})
		if actualRequest == arbitraryNotFoundHash.ToBase64() {
			response.WriteHeader(http.StatusNotFound)
		} else {
			var data []byte
			if actualRequest == arbitraryHashNoPrivateMetadata.ToBase64() {
				data, _ = json.Marshal(&receiveResponse{
					Payload: arbitraryPrivatePayload,
				})
			} else {
				data, _ = json.Marshal(&receiveResponse{
					Payload: arbitraryPrivatePayload,
				})
			}
			response.Write(data)
		}
	}
}

func MockSendSignedTxOctetStreamAPIHandlerFunc(response http.ResponseWriter, request *http.Request) {
	actualRequest := new(sendSignedTxRequest)
	reqHash, err := ioutil.ReadAll(request.Body)
	if err != nil {
		go func(o *capturedRequest) { sendSignedTxOctetStreamRequestCaptor <- o }(&capturedRequest{err: err})
		return
	}
	actualRequest.Hash = reqHash
	actualRequest.To = strings.Split(request.Header["C11n-To"][0], ",")

	go func(o *capturedRequest) { sendSignedTxOctetStreamRequestCaptor <- o }(&capturedRequest{request: actualRequest, header: request.Header})
	response.Write([]byte(common.BytesToEncryptedPayloadHash(reqHash).ToBase64()))
}

func teardown() {
	testServer.Close()
}

func verifyRequestHeader(h http.Header, t *testing.T) {
	assert.Equal(t, "application/json", h.Get("Content-type"))
	assert.Equal(t, "application/json", h.Get("Accept"))
}

func TestSend_whenTypical(t *testing.T) {
	actualHash, err := testObject.Send(arbitraryPrivatePayload, arbitraryFrom, arbitraryTo, standardPrivateExtra)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-sendRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	verifyRequestHeader(capturedRequest.header, t)
	actualRequest := capturedRequest.request.(*sendRequest)

	assert.Equal(t, arbitraryPrivatePayload, actualRequest.Payload, "request.payload")
	assert.Equal(t, arbitraryFrom, actualRequest.From, "request.from")
	assert.Equal(t, arbitraryTo, actualRequest.To, "request.to")
	assert.Equal(t, arbitraryHash, actualHash, "returned hash")
}

func TestSend_whenHttpFailure(t *testing.T) {
	mux := http.NewServeMux()
	testServerNoSendMethod := httptest.NewServer(mux)
	defer testServerNoSendMethod.Close()

	testObjectNoSendMethod := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServerNoSendMethod.URL,
	})

	_, err := testObjectNoSendMethod.Send(arbitraryPrivatePayload, arbitraryFrom, arbitraryTo, standardPrivateExtra)
	assert.EqualError(t, err, "404 status: 404 page not found\n")
}

func TestSend_whenUnknownReturnValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/send", func(response http.ResponseWriter, request *http.Request) {
		data, _ := json.Marshal(&sendResponse{Key: "some random non-b64 data"})
		response.Write(data)
	})
	testServerBadData := httptest.NewServer(mux)
	defer testServerBadData.Close()

	testObjectBadData := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServerBadData.URL,
	})

	_, err := testObjectBadData.Send(arbitraryPrivatePayload, arbitraryFrom, arbitraryTo, standardPrivateExtra)
	assert.EqualError(t, err, "unable to decode encrypted payload hash: some random non-b64 data. Cause: unable to convert base64 string some random non-b64 data to EncryptedPayloadHash. Cause: illegal base64 data at input byte 4")
}

func TestSend_whenTesseraVersionDoesNotSupportPrivacyEnhancements(t *testing.T) {
	testObjectNoPE := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.False(t, testObjectNoPE.HasFeature(engine.PrivacyEnhancements))

	nonStandardPrivateExtra := &engine.ExtraMetadata{
		PrivacyFlag: arbitraryPrivacyFlag,
	}

	// trying to send a party protection transaction
	_, err := testObjectNoPE.Send(arbitraryPrivatePayload, arbitraryFrom, arbitraryTo, nonStandardPrivateExtra)
	assert.EqualError(t, err, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements.Error())
}

func TestTesseraPrivateTxManager_EncryptPayload(t *testing.T) {
	_, err := testObject.EncryptPayload([]byte("some data"), "privateFromKey", []string{}, nil)
	assert.EqualError(t, err, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements.Error())
}

func TestTesseraPrivateTxManager_DecryptPayload(t *testing.T) {
	_, _, err := testObject.DecryptPayload(common.DecryptRequest{})
	assert.EqualError(t, err, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements.Error())
}

func TestTesseraPrivateTxManager_ReceiveRaw(t *testing.T) {
	_, _, err := testObject.ReceiveRaw(common.EncryptedPayloadHash{})
	assert.EqualError(t, err, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements.Error())
}

func TestStoreRaw_whenTypical(t *testing.T) {
	actualHash, err := testObject.StoreRaw(arbitraryPrivatePayload, arbitraryFrom)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-sendRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	verifyRequestHeader(capturedRequest.header, t)
	actualRequest := capturedRequest.request.(*storerawRequest)

	assert.Equal(t, arbitraryPrivatePayload, actualRequest.Payload, "request.payload")
	assert.Equal(t, arbitraryFrom, actualRequest.From, "request.from")
	assert.Equal(t, arbitraryHash, actualHash, "returned hash")
}

func TestStoreRaw_whenHttpFailure(t *testing.T) {
	mux := http.NewServeMux()
	testServerNoSendMethod := httptest.NewServer(mux)
	defer testServerNoSendMethod.Close()

	testObjectNoSendMethod := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServerNoSendMethod.URL,
	})

	_, err := testObjectNoSendMethod.StoreRaw(arbitraryPrivatePayload, arbitraryFrom)
	assert.EqualError(t, err, "404 status: 404 page not found\n")
}

func TestStoreRaw_whenUnknownReturnValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/storeraw", func(response http.ResponseWriter, request *http.Request) {
		actualRequest := new(storerawRequest)
		go func(o *capturedRequest) {
			sendRequestCaptor <- o
		}(&capturedRequest{request: actualRequest, header: request.Header})

		data, _ := json.Marshal(&sendResponse{Key: "some random non-b64 data"})
		response.Write(data)
	})
	testServerBadData := httptest.NewServer(mux)
	defer testServerBadData.Close()

	testObjectBadData := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServerBadData.URL,
	})

	_, err := testObjectBadData.StoreRaw(arbitraryPrivatePayload, arbitraryFrom)
	assert.EqualError(t, err, "unable to decode encrypted payload hash: some random non-b64 data. Cause: unable to convert base64 string some random non-b64 data to EncryptedPayloadHash. Cause: illegal base64 data at input byte 4")
}

func TestReceive_whenTypical(t *testing.T) {
	_, _, err := testObject.Receive(arbitraryHash1)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-receiveRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	verifyRequestHeader(capturedRequest.header, t)
	actualRequest := capturedRequest.request.(string)
	assert.Equal(t, arbitraryHash1.ToBase64(), actualRequest, "requested hash")
}

func TestReceive_whenPayloadNotFound(t *testing.T) {
	data, _, err := testObject.Receive(arbitraryNotFoundHash)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-receiveRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	verifyRequestHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(string)

	assert.Equal(t, arbitraryNotFoundHash.ToBase64(), actualRequest, "requested hash")
	assert.Nil(t, data, "returned payload when not found")
}

func TestReceive_whenEncryptedPayloadHashIsEmpty(t *testing.T) {
	data, _, err := testObject.Receive(emptyHash)
	if !assert.Nil(t, err) {
		t.Fatal()
	}
	assert.Empty(t, receiveRequestCaptor, "no request is actually sent")
	assert.Nil(t, data, "returned payload when not found")
}

func TestReceive_whenHavingPayloadButNoPrivateExtraMetadata(t *testing.T) {
	_, actualExtra, err := testObject.Receive(arbitraryHashNoPrivateMetadata)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-receiveRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	verifyRequestHeader(capturedRequest.header, t)

	actualRequest := capturedRequest.request.(string)

	assert.Equal(t, arbitraryHashNoPrivateMetadata.ToBase64(), actualRequest, "requested hash")
	assert.Empty(t, actualExtra.ACHashes, "returned affected contract transaction hashes")
	assert.True(t, common.EmptyHash(actualExtra.ACMerkleRoot), "returned merkle root")
}

func TestSendSignedTx_whenTypical(t *testing.T) {
	_, err := testObject.SendSignedTx(arbitraryHash, arbitraryTo, standardPrivateExtra)
	if !assert.Nil(t, err) {
		t.Fatal()
	}

	capturedRequest := <-sendSignedTxOctetStreamRequestCaptor
	if !assert.Nil(t, capturedRequest.err) {
		t.Fatal()
	}

	actualRequest := capturedRequest.request.(*sendSignedTxRequest)
	assert.Equal(t, arbitraryTo, actualRequest.To, "request.to")
}

func TestSendSignedTx_whenNotStandardPrivateTx(t *testing.T) {
	partyProtectionExtraData := &engine.ExtraMetadata{PrivacyFlag: engine.PrivacyFlagPartyProtection}
	_, err := testObject.SendSignedTx(arbitraryHash, arbitraryTo, partyProtectionExtraData)
	assert.EqualError(t, err, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements.Error())
}
