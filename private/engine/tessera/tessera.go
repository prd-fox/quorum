package tessera

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/private/engine"
	"github.com/ethereum/go-ethereum/private/engine/tessera/version1_0"
	"github.com/ethereum/go-ethereum/private/engine/tessera/version2_0"
)

type Identifiable interface {
	Name() string
	HasFeature(f engine.PrivateTransactionManagerFeature) bool
}

// Interacting with Private Transaction Manager APIs
type PrivateTransactionManager interface {
	Identifiable

	Send(data []byte, from string, to []string, extra *engine.ExtraMetadata) (common.EncryptedPayloadHash, error)
	StoreRaw(data []byte, from string) (common.EncryptedPayloadHash, error)
	SendSignedTx(data common.EncryptedPayloadHash, to []string, extra *engine.ExtraMetadata) ([]byte, error)
	// Returns nil payload if not found
	Receive(data common.EncryptedPayloadHash) ([]byte, *engine.ExtraMetadata, error)
	// Returns nil payload if not found
	ReceiveRaw(data common.EncryptedPayloadHash) ([]byte, *engine.ExtraMetadata, error)
	IsSender(txHash common.EncryptedPayloadHash) (bool, error)
	GetParticipants(txHash common.EncryptedPayloadHash) ([]string, error)
	EncryptPayload(data []byte, from string, to []string, extra *engine.ExtraMetadata) ([]byte, error)
	DecryptPayload(payload common.DecryptRequest) ([]byte, *engine.ExtraMetadata, error)
}

func New(client *engine.Client) (PrivateTransactionManager, error) {
	highestKnownVersion := RetrieveTesseraAPIVersion(client)

	switch highestKnownVersion {
	case apiVersion1:
		return version1_0.New(client), nil
	case apiVersion2:
		return version2_0.New(client), nil
	default:
		return nil, errors.New("no known version of tessera")
	}
}
