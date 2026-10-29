// Package swish builds the links and QR codes that open the Swish app with a
// prefilled payment.
package swish

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
)

// Payment is what Swish gets prefilled with. Only the message can be edited
// in the app.
type Payment struct {
	Payee   string
	Amount  int32
	Message string
}

type stringValue struct {
	Value    string `json:"value"`
	Editable bool   `json:"editable"`
}

type intValue struct {
	Value    int32 `json:"value"`
	Editable bool  `json:"editable"`
}

// uriData is the payload of a swish://payment link. The field order matches
// what the Kotlin service sent.
type uriData struct {
	Version int         `json:"version"`
	Payee   stringValue `json:"payee"`
	Amount  intValue    `json:"amount"`
	Message stringValue `json:"message"`
}

// URI returns the swish://payment link that opens the Swish app with p.
func (p Payment) URI() string {
	data := uriData{
		Version: 1,
		Payee:   stringValue{Value: p.Payee},
		Amount:  intValue{Value: p.Amount},
		Message: stringValue{Value: p.Message, Editable: true},
	}
	// QueryEscape writes spaces as +, which only form decoding turns back
	// into spaces. Use %20 so any decoder gets them right. Real + signs are
	// already %2B at this point.
	escaped := strings.ReplaceAll(url.QueryEscape(string(marshal(data))), "+", "%20")
	return "swish://payment?data=" + escaped
}

// marshal encodes v as JSON without escaping <, > and &, like Jackson does.
func marshal(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	// Encoding plain structs of strings and integers cannot fail.
	_ = enc.Encode(v)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}
