package version1_0

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/private/cache"
	"github.com/ethereum/go-ethereum/private/engine"
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

func (t *tesseraPrivateTxManager) submitJSON(method, path string, request interface{}, response interface{}) (int, error) {
	req, err := newOptionalJSONRequest(method, t.client.FullPath(path), request)
	if err != nil {
		return -1, fmt.Errorf("unable to build json request for (method:%s, path:%s). Cause: %v", method, path, err)
	}
	res, err := t.client.HttpClient.Do(req)
	if err != nil {
		return -1, fmt.Errorf("unable to submit request (method:%s, path:%s). Cause: %v", method, path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(res.Body)
		return res.StatusCode, fmt.Errorf("%d status: %s", res.StatusCode, string(body))
	}
	if err := json.NewDecoder(res.Body).Decode(response); err != nil {
		return res.StatusCode, fmt.Errorf("unable to decode response body for (method:%s, path:%s). Cause: %v", method, path, err)
	}
	return res.StatusCode, nil
}

func (t *tesseraPrivateTxManager) Send(data []byte, from string, to []string, extra *engine.ExtraMetadata) (common.EncryptedPayloadHash, error) {
	if extra.PrivacyFlag.IsNotStandardPrivate() {
		return common.EncryptedPayloadHash{}, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements
	}

	requestObject := &sendRequest{
		Payload: data,
		From:    from,
		To:      to,
	}

	response := new(sendResponse)
	if _, err := t.submitJSON("POST", "/send", requestObject, response); err != nil {
		return common.EncryptedPayloadHash{}, err
	}

	eph, err := common.Base64ToEncryptedPayloadHash(response.Key)
	if err != nil {
		return common.EncryptedPayloadHash{}, fmt.Errorf("unable to decode encrypted payload hash: %s. Cause: %v", response.Key, err)
	}

	cacheKey := eph.Hex()
	cacheValue := cache.PrivateCacheItem{
		Payload: data,
		Extra:   *extra,
	}
	t.cache.Set(cacheKey, cacheValue, gocache.DefaultExpiration)

	return eph, nil
}

func (t *tesseraPrivateTxManager) StoreRaw(data []byte, from string) (common.EncryptedPayloadHash, error) {
	response := new(sendResponse)

	request := &storerawRequest{
		Payload: data,
		From:    from,
	}
	if _, err := t.submitJSON("POST", "/storeraw", request, response); err != nil {
		return common.EncryptedPayloadHash{}, err
	}

	eph, err := common.Base64ToEncryptedPayloadHash(response.Key)
	if err != nil {
		return common.EncryptedPayloadHash{}, fmt.Errorf("unable to decode encrypted payload hash: %s. Cause: %v", response.Key, err)
	}

	var extra engine.ExtraMetadata
	cacheKeyTemp := fmt.Sprintf("%s-incomplete", eph.Hex())
	cacheValue := cache.PrivateCacheItem{
		Payload: data,
		Extra:   extra,
	}
	t.cache.Set(cacheKeyTemp, cacheValue, gocache.DefaultExpiration)

	return eph, nil
}

// allow new quorum to send raw transactions when connected to an old tessera
func (t *tesseraPrivateTxManager) sendSignedPayloadOctetStream(signedPayload []byte, b64To []string) ([]byte, error) {
	buf := bytes.NewBuffer(signedPayload)
	req, err := http.NewRequest("POST", t.client.FullPath("/sendsignedtx"), buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("c11n-to", strings.Join(b64To, ","))
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

	return ioutil.ReadAll(res.Body)
}

// also populate cache item with additional extra metadata
func (t *tesseraPrivateTxManager) SendSignedTx(data common.EncryptedPayloadHash, to []string, extra *engine.ExtraMetadata) ([]byte, error) {
	if extra.PrivacyFlag.IsNotStandardPrivate() {
		return nil, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements
	}

	returnedHash, err := t.sendSignedPayloadOctetStream(data.Bytes(), to)
	if err != nil {
		return nil, err
	}

	hashBytes, err := base64.StdEncoding.DecodeString(string(returnedHash))
	if err != nil {
		return nil, err
	}
	// pull incomplete cache item and inject new cache item with complete information
	cacheKey := data.Hex()
	cacheKeyTemp := fmt.Sprintf("%s-incomplete", cacheKey)
	if item, found := t.cache.Get(cacheKeyTemp); found {
		if incompleteCacheItem, ok := item.(cache.PrivateCacheItem); ok {
			t.cache.Set(cacheKey, cache.PrivateCacheItem{
				Payload: incompleteCacheItem.Payload,
				Extra:   *extra,
			}, gocache.DefaultExpiration)
			t.cache.Delete(cacheKeyTemp)
		}
	}
	return hashBytes, err
}

func (t *tesseraPrivateTxManager) Receive(data common.EncryptedPayloadHash) ([]byte, *engine.ExtraMetadata, error) {
	if common.EmptyEncryptedPayloadHash(data) {
		return nil, nil, nil
	}

	cacheKey := data.Hex()
	if item, found := t.cache.Get(cacheKey); found {
		cacheItem, ok := item.(cache.PrivateCacheItem)
		if !ok {
			return nil, nil, fmt.Errorf("unknown cache item. expected type PrivateCacheItem")
		}
		return cacheItem.Payload, &cacheItem.Extra, nil
	}

	response := new(receiveResponse)
	formattedUrl := fmt.Sprintf("/transaction/%s", url.PathEscape(data.ToBase64()))
	if statusCode, err := t.submitJSON("GET", formattedUrl, nil, response); err != nil {
		if statusCode == http.StatusNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	var extra engine.ExtraMetadata
	t.cache.Set(cacheKey, cache.PrivateCacheItem{
		Payload: response.Payload,
		Extra:   extra,
	}, gocache.DefaultExpiration)

	return response.Payload, &extra, nil
}

func (t *tesseraPrivateTxManager) ReceiveRaw(_ common.EncryptedPayloadHash) ([]byte, *engine.ExtraMetadata, error) {
	return nil, nil, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements
}

func (t *tesseraPrivateTxManager) EncryptPayload(_ []byte, _ string, _ []string, _ *engine.ExtraMetadata) ([]byte, error) {
	return nil, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements
}

func (t *tesseraPrivateTxManager) DecryptPayload(_ common.DecryptRequest) ([]byte, *engine.ExtraMetadata, error) {
	return nil, nil, engine.ErrPrivateTxManagerDoesNotSupportPrivacyEnhancements
}

func (t *tesseraPrivateTxManager) IsSender(txHash common.EncryptedPayloadHash) (bool, error) {
	formattedUrl := "http+unix://c/transaction/" + url.PathEscape(txHash.ToBase64()) + "/isSender"

	req, err := http.NewRequest("GET", formattedUrl, nil)
	if err != nil {
		return false, err
	}

	res, err := t.client.HttpClient.Do(req)

	if res != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return false, err
	}

	if res.StatusCode != 200 {
		return false, fmt.Errorf("non-200 status code: %+v", res)
	}

	out, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	return strconv.ParseBool(string(out))
}

func (t *tesseraPrivateTxManager) GetParticipants(txHash common.EncryptedPayloadHash) ([]string, error) {
	requestUrl := "http+unix://c/transaction/" + url.PathEscape(txHash.ToBase64()) + "/participants"
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, err
	}

	res, err := t.client.HttpClient.Do(req)

	if res != nil {
		defer res.Body.Close()
	}

	if err != nil {
		return nil, err
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 status code: %+v", res)
	}

	out, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(out), ","), nil
}

func (t *tesseraPrivateTxManager) Name() string {
	return "Tessera - API v1.0"
}

func (t *tesseraPrivateTxManager) HasFeature(_ engine.PrivateTransactionManagerFeature) bool {
	return false
}

// don't serialize body if nil
func newOptionalJSONRequest(method string, path string, body interface{}) (*http.Request, error) {
	buf := new(bytes.Buffer)
	if body != nil {
		if err := json.NewEncoder(buf).Encode(body); err != nil {
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
