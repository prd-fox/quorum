package constellation

import (
	"fmt"

	"github.com/ethereum/go-ethereum/private/engine"

	"github.com/ethereum/go-ethereum/private/cache"

	gocache "github.com/patrickmn/go-cache"
)

type constellation struct {
	node *Client
	c    *gocache.Cache
}

func Is(ptm interface{}) bool {
	_, ok := ptm.(*constellation)
	return ok
}

func New(client *engine.Client) *constellation {
	return &constellation{
		node: &Client{
			httpClient: client.HttpClient,
		},
		c: cache.NewDefaultCache(),
	}
}

func (g *constellation) Send(data []byte, from string, to []string) ([]byte, error) {
	out, err := g.node.SendPayload(data, from, to)
	if err != nil {
		return nil, err
	}
	cacheKey := string(out)
	g.c.Set(cacheKey, data, cache.DefaultExpiration)
	return out, nil
}

func (g *constellation) SendSignedTx(data []byte, to []string) (out []byte, err error) {
	return nil, engine.ErrPrivateTxManagerNotSupported
}

func (g *constellation) Receive(key []byte) ([]byte, error) {
	// Ignore this error since not being a recipient of
	// a payload isn't an error.
	// TODO: Return an error if it's anything OTHER than
	// 'you are not a recipient.'
	cacheKey := string(key)
	x, found := g.c.Get(cacheKey)
	if found {
		cacheItem, ok := x.([]byte)
		if !ok {
			return nil, fmt.Errorf("unknown cache item. expected type PrivateCacheItem")
		}
		return cacheItem, nil
	}
	privatePayload, _ := g.node.ReceivePayload(key)
	g.c.Set(cacheKey, privatePayload, cache.DefaultExpiration)
	return privatePayload, nil
}

func (g *constellation) Name() string {
	return "Constellation"
}
