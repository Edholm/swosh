package swish

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// DefaultQREndpoint is Swish's public API for prefilled QR codes.
const DefaultQREndpoint = "https://mpc.getswish.net/qrg-swish/api/v1/prefilled"

const (
	qrCacheSize = 1024
	// maxQRSize caps how much of a response is read. The PNGs are a few KB.
	maxQRSize = 1 << 20
)

// QRClient fetches PNG QR codes from Swish and keeps recent ones in memory.
type QRClient struct {
	endpoint string
	http     *http.Client
	cache    *lru.Cache[Payment, []byte]
}

// NewQRClient returns a client for the Swish QR API at endpoint.
func NewQRClient(endpoint string) *QRClient {
	// lru.New only fails for a non-positive size.
	cache, _ := lru.New[Payment, []byte](qrCacheSize)
	return &QRClient{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 10 * time.Second},
		cache:    cache,
	}
}

type qrRequest struct {
	Payee       stringValue `json:"payee"`
	Amount      intValue    `json:"amount"`
	Message     stringValue `json:"message"`
	Format      string      `json:"format"`
	Size        int         `json:"size"`
	Border      int         `json:"border"`
	Transparent bool        `json:"transparent"`
}

// PNG returns a QR code for p that the Swish app can scan.
func (c *QRClient) PNG(ctx context.Context, p Payment) ([]byte, error) {
	if png, ok := c.cache.Get(p); ok {
		return png, nil
	}

	body := marshal(qrRequest{
		Payee:       stringValue{Value: p.Payee, Editable: p.PayeeEditable},
		Amount:      intValue{Value: p.Amount, Editable: p.AmountEditable},
		Message:     stringValue{Value: p.Message, Editable: p.MessageEditable},
		Format:      "png",
		Size:        300,
		Border:      2,
		Transparent: true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build QR request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch QR code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch QR code: Swish answered %s", resp.Status)
	}
	png, err := io.ReadAll(io.LimitReader(resp.Body, maxQRSize))
	if err != nil {
		return nil, fmt.Errorf("read QR code: %w", err)
	}

	c.cache.Add(p, png)
	return png, nil
}
