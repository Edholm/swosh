package swish

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestURI(t *testing.T) {
	tests := []struct {
		name string
		p    Payment
		want string
	}{
		{
			name: "plain",
			p:    Payment{Payee: "070-000 00 00", Amount: 100, Message: "msg", MessageEditable: true},
			want: "swish://payment?data=%7B%22version%22%3A1%2C%22payee%22%3A%7B%22value%22%3A%22070-000%2000%2000%22%2C%22editable%22%3Afalse%7D%2C" +
				"%22amount%22%3A%7B%22value%22%3A100%2C%22editable%22%3Afalse%7D%2C%22message%22%3A%7B%22value%22%3A%22msg%22%2C%22editable%22%3Atrue%7D%7D",
		},
		{
			// Characters Go's JSON encoder would escape for HTML, query
			// delimiters, and non-ASCII.
			name: "special characters",
			p:    Payment{Payee: "1230000000", Amount: 1, Message: "Öl & <mat>?#1+1=2", MessageEditable: true},
			want: "swish://payment?data=%7B%22version%22%3A1%2C%22payee%22%3A%7B%22value%22%3A%221230000000%22%2C%22editable%22%3Afalse%7D%2C" +
				"%22amount%22%3A%7B%22value%22%3A1%2C%22editable%22%3Afalse%7D%2C%22message%22%3A%7B%22value%22%3A%22%C3%96l%20%26%20%3Cmat%3E%3F%231%2B1%3D2%22%2C%22editable%22%3Atrue%7D%7D",
		},
		{
			name: "payee and amount editable",
			p:    Payment{Payee: "1230000000", Amount: 1, PayeeEditable: true, AmountEditable: true},
			want: "swish://payment?data=%7B%22version%22%3A1%2C%22payee%22%3A%7B%22value%22%3A%221230000000%22%2C%22editable%22%3Atrue%7D%2C" +
				"%22amount%22%3A%7B%22value%22%3A1%2C%22editable%22%3Atrue%7D%2C%22message%22%3A%7B%22value%22%3A%22%22%2C%22editable%22%3Afalse%7D%7D",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.URI(); got != tt.want {
				t.Errorf("URI() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// TestURIRoundTrip checks that a query parser gets the message back intact,
// which is what broke for & in issue #13.
func TestURIRoundTrip(t *testing.T) {
	for _, msg := range []string{"Öl & mat", "1+1=2", "a&data=b", "100% #1?"} {
		p := Payment{Payee: "1230000000", Amount: 1, Message: msg}
		u, err := url.Parse(p.URI())
		if err != nil {
			t.Fatalf("parse URI for %q: %v", msg, err)
		}
		var got uriData
		if err := json.Unmarshal([]byte(u.Query().Get("data")), &got); err != nil {
			t.Fatalf("decode data for %q: %v", msg, err)
		}
		if got.Message.Value != msg {
			t.Errorf("message = %q, want %q", got.Message.Value, msg)
		}
	}
}

func TestQRClientPNG(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		var got qrRequest
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode request %s: %v", body, err)
		}
		want := qrRequest{
			Payee:       stringValue{Value: "070-000 00 00"},
			Amount:      intValue{Value: 100, Editable: true},
			Message:     stringValue{Value: "msg"},
			Format:      "png",
			Size:        300,
			Border:      2,
			Transparent: true,
		}
		if got != want {
			t.Errorf("request = %+v, want %+v", got, want)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png"))
	}))
	defer srv.Close()

	c := NewQRClient(srv.URL)
	p := Payment{Payee: "070-000 00 00", Amount: 100, Message: "msg", AmountEditable: true}
	for range 2 {
		png, err := c.PNG(t.Context(), p)
		if err != nil {
			t.Fatalf("PNG: %v", err)
		}
		if string(png) != "png" {
			t.Errorf("PNG = %q, want %q", png, "png")
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("Swish was called %d times, want 1 (second call should be cached)", n)
	}
}

func TestQRClientPNGError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewQRClient(srv.URL)
	if _, err := c.PNG(t.Context(), Payment{Payee: "x", Amount: 1}); err == nil {
		t.Error("PNG with a failing Swish API: got nil error")
	}
}
