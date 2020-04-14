package engine

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/tv42/httpunix"
)

var (
	ErrPrivateTxManagerNotReady     = errors.New("private transaction manager is not ready")
	ErrPrivateTxManagerNotSupported = errors.New("private transaction manager does not suppor this operation")
)

type Client struct {
	HttpClient *http.Client
	BaseURL    string
}

func NewClient(socketPath string) *Client {
	return &Client{
		HttpClient: &http.Client{
			Transport: unixTransport(socketPath),
		},
		BaseURL: "http+unix://c",
	}
}

func unixTransport(socketPath string) *httpunix.Transport {
	t := &httpunix.Transport{
		DialTimeout:           1 * time.Second,
		RequestTimeout:        5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}
	t.RegisterLocation("c", socketPath)
	return t
}

func (c *Client) FullPath(path string) string {
	return fmt.Sprintf("%s%s", c.BaseURL, path)
}

func (c *Client) Get(path string) (*http.Response, error) {
	return c.HttpClient.Get(c.FullPath(path))
}
