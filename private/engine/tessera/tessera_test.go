package tessera

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/private/engine"
	"github.com/stretchr/testify/assert"
)

func TestNew_version1(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(response http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal([]string{"1.0"})
		response.Write(data)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	testClient, err := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Nil(t, err)
	assert.Equal(t, "Tessera - API v1.0", testClient.Name())
}

func TestNew_version1FromNone(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(response http.ResponseWriter, _ *http.Request) {
		response.Write([]byte(`[]`))
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	testClient, err := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Nil(t, err)
	assert.Equal(t, "Tessera - API v1.0", testClient.Name())
}

func TestNew_version2(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(response http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal([]string{"2.0"})
		response.Write(data)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	testClient, err := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Nil(t, err)
	assert.Equal(t, "Tessera - API v2.0", testClient.Name())
}

func TestNew_version2FromMixed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(response http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal([]string{"1.0", "2.0"})
		response.Write(data)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	testClient, err := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Nil(t, err)
	assert.Equal(t, "Tessera - API v2.0", testClient.Name())
}

func TestNew_versionUnknown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(response http.ResponseWriter, _ *http.Request) {
		data, _ := json.Marshal([]string{"0.1"})
		response.Write(data)
	})

	testServer := httptest.NewServer(mux)
	defer testServer.Close()

	testClient, err := New(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Nil(t, testClient)
	assert.EqualError(t, err, "no known version of tessera")
}
