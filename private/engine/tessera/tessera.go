package tessera

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/private/engine"

	"github.com/ethereum/go-ethereum/private/cache"

	"github.com/ethereum/go-ethereum/params"

	gocache "github.com/patrickmn/go-cache"
)

type tesseraPrivateTxManager struct {
	client *engine.Client
	cache  *gocache.Cache
}

func Is(ptm interface{}) bool {
	_, ok := ptm.(*tesseraPrivateTxManager)
	return ok
}

func New(client *engine.Client) *tesseraPrivateTxManager {
	return &tesseraPrivateTxManager{
		client: client,
		cache:  cache.NewDefaultCache(),
	}
}

func (t *tesseraPrivateTxManager) Send(data []byte, from string, to []string) ([]byte, error) {
	response := new(sendResponse)
	if _, err := t.submitJSON("POST", "/send", &sendRequest{
		Payload: data,
		From:    from,
		To:      to,
	}, response); err != nil {
		return nil, err
	}

	hsh, err := base64.StdEncoding.DecodeString(response.Key)
	if err != nil {
		return nil, err
	}

	t.cache.Set(string(hsh), data, gocache.DefaultExpiration)

	return hsh, nil
}

// also populate cache item with additional extra metadata
func (t *tesseraPrivateTxManager) SendSignedTx(data []byte, to []string) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	req, err := http.NewRequest("POST", "http+unix://c/sendsignedtx", buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("c11n-to", strings.Join(to, ","))
	req.Header.Set("Content-Type", "application/octet-stream")
	res, err := t.client.HttpClient.Do(req)

	if res != nil {
		defer res.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Non-200 status code: %+v", res)
	}

	return ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, res.Body))
}

func (t *tesseraPrivateTxManager) Receive(data []byte) ([]byte, error) {
	return t.receive(data, false)
}

// retrieve raw will not return information about medata
func (t *tesseraPrivateTxManager) receive(key []byte, isRaw bool) ([]byte, error) {
	if item, found := t.cache.Get(string(key)); found {
		cacheItem, ok := item.([]byte)
		if !ok {
			return nil, fmt.Errorf("unknown cache item. expected type []byte")
		}
		return cacheItem, nil
	}

	response := new(receiveResponse)
	if statusCode, err := t.submitJSON("GET", fmt.Sprintf("/transaction/%s?isRaw=%v", url.PathEscape(base64.StdEncoding.EncodeToString(key)), isRaw), nil, response); err != nil {
		if statusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	dataBytes, err := base64.StdEncoding.DecodeString(response.Payload)
	if err != nil {
		return nil, err
	}
	t.cache.Set(string(key), dataBytes, gocache.DefaultExpiration)

	return dataBytes, nil
}

func (t *tesseraPrivateTxManager) Name() string {
	return "Tessera"
}

func (t *tesseraPrivateTxManager) submitJSON(method, path string, request interface{}, response interface{}) (int, error) {
	req, err := newOptionalJSONRequest(method, t.client.FullPath(path), request)
	if err != nil {
		return -1, err
	}
	res, err := t.client.HttpClient.Do(req)
	if err != nil {
		return -1, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(res.Body)
		return res.StatusCode, fmt.Errorf("%d status: %s", res.StatusCode, string(body))
	}
	if err := json.NewDecoder(res.Body).Decode(response); err != nil {
		return res.StatusCode, err
	}
	return res.StatusCode, nil
}

// don't serialize body if nil
func newOptionalJSONRequest(method string, path string, body interface{}) (*http.Request, error) {
	buf := new(bytes.Buffer)
	if body != nil {
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}
	request, err := http.NewRequest(method, path, buf)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", fmt.Sprintf("quorum-v%s", params.QuorumVersion))
	request.Header.Set("Content-type", "application/json")
	request.Header.Set("Accept", "application/json")
	return request, nil
}
