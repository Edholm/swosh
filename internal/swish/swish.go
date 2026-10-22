// Package swish builds the links and QR codes that open the Swish app with a
// prefilled payment.
package swish

import (
	"bytes"
	"encoding/json"
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
	return "swish://payment?data=" + escapeFragment(string(marshal(data)))
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

// escapeFragment percent-encodes s for a URI fragment the way Guava's
// UrlEscapers.urlFragmentEscaper does, which is what the Kotlin service used.
func escapeFragment(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if fragmentSafe(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xf])
	}
	return b.String()
}

func fragmentSafe(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("-._~!$&'()*+,;=:@/?", c) >= 0
}
