package tessera

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/private/engine"
	"github.com/stretchr/testify/assert"
)

var testServer *httptest.Server

func TestVersionApi_404NotFound(t *testing.T) {
	mux := http.NewServeMux()

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, apiVersion1, version)
}

func TestVersionApi_GarbageData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("GARBAGE"))
	})

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, unknownApiVersion, version)
}

func TestVersionApi_emptyVersionsArray(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("[]"))
	})

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, apiVersion1, version)
}

func TestVersionApi_invalidVersionItem(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`["1.0", ""]`))
	})

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, apiVersion1, version)
}

func TestVersionApi_validVersionInWrongOrder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("[\"2.0\",\"1.0\"]"))
	})

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, "2.0", version)
}

func TestVersionApi_validVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/version/api", func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("[\"1.0\",\"2.0\"]"))
	})

	testServer = httptest.NewServer(mux)
	defer testServer.Close()

	version := RetrieveTesseraAPIVersion(&engine.Client{
		HttpClient: &http.Client{},
		BaseURL:    testServer.URL,
	})

	assert.Equal(t, "2.0", version)
}
