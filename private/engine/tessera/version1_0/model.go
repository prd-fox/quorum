package version1_0

// request object for /send API
type sendRequest struct {
	Payload []byte `json:"payload"`
	// base64-encoded
	From string   `json:"from,omitempty"`
	To   []string `json:"to"`
}

// request object for /send API
type storerawRequest struct {
	Payload []byte `json:"payload"`
	// base64-encoded
	From string `json:"from,omitempty"`
}

// response object for /send API
type sendResponse struct {
	// Base64-encoded
	Key string `json:"key"`
}

type receiveResponse struct {
	Payload []byte `json:"payload"`
}

type sendSignedTxRequest struct {
	Hash []byte   `json:"hash"`
	To   []string `json:"to"`
}

type sendSignedTxResponse struct {
	// Base64-encoded
	Key string `json:"key"`
}
