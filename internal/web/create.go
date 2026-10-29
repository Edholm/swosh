package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/nyaruka/phonenumbers"
)

const (
	// defaultExpiry applies when a request leaves out expireAfterSeconds.
	defaultExpiry = 2 * 24 * time.Hour
	// maxExpiry keeps expiry times far inside what Postgres can store.
	maxExpiry = 100 * 365 * 24 * time.Hour
	// maxAmount is the most anyone can Swish in one payment, in kronor.
	maxAmount          = 150_000
	maxMessageLength   = 50
	swedenCountryCode  = 46
	swishNumberPrefix  = "123"
	swishNumberLength  = 10
	invalidInputReason = "Invalid input format!"
)

// createRequest is the body of POST /api/create.
type createRequest struct {
	Phone   *string     `json:"phone"`
	Amount  nullableInt `json:"amount"`
	Message *string     `json:"message"`
	// Missing means defaultExpiry; null or 0 means the link never expires.
	ExpireAfterSeconds nullableInt `json:"expireAfterSeconds"`
}

// nullableInt is an integer that may also be sent as a string, the way the
// web form does. Like Jackson, it reads null and "" as no value.
type nullableInt struct {
	present bool // the field was in the JSON object
	valid   bool // the field had a value
	value   int64
}

func (n *nullableInt) UnmarshalJSON(b []byte) error {
	n.present = true
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	var s string
	if len(b) > 0 && b[0] == '"' {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "" {
			return nil
		}
	} else {
		s = string(b)
	}
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return fmt.Errorf("not an integer: %s", b)
	}
	n.valid, n.value = true, v
	return nil
}

// newSwosh is a validated link that is ready to be stored.
type newSwosh struct {
	Payee       string
	Amount      int32
	Description *string
	ExpiresOn   *time.Time
}

// validationError is a reason shown to the API caller.
type validationError string

func (e validationError) Error() string { return string(e) }

func invalid(format string, args ...any) error {
	return validationError(fmt.Sprintf(format, args...))
}

// validate checks req in the same order and with the same messages as the
// Kotlin service, which had no upper limit on the amount, and returns the
// link to store.
func (req createRequest) validate(now time.Time) (newSwosh, error) {
	if !req.Amount.valid || req.Phone == nil || strings.TrimSpace(*req.Phone) == "" {
		return newSwosh{}, invalid("Missing input parameters. 'phone' and 'amount' is required")
	}
	if req.Amount.value < 1 {
		return newSwosh{}, invalid("Minimum allowed amount is 1. Got %d", req.Amount.value)
	}
	if req.Amount.value > maxAmount {
		return newSwosh{}, invalid("Maximum allowed amount is %d. Got %d", maxAmount, req.Amount.value)
	}
	var description *string
	if req.Message != nil {
		// Kotlin counted UTF-16 code units.
		if n := len(utf16.Encode([]rune(*req.Message))); n > maxMessageLength {
			return newSwosh{}, invalid("Description is too long. Max %d chars. Got %d", maxMessageLength, n)
		}
		if strings.TrimSpace(*req.Message) != "" {
			description = req.Message
		}
	}
	payee, err := formatPayee(*req.Phone)
	if err != nil {
		return newSwosh{}, err
	}
	expiresOn, err := req.expiresOn(now)
	if err != nil {
		return newSwosh{}, err
	}
	return newSwosh{
		Payee:       payee,
		Amount:      int32(req.Amount.value),
		Description: description,
		ExpiresOn:   expiresOn,
	}, nil
}

func (req createRequest) expiresOn(now time.Time) (*time.Time, error) {
	after := defaultExpiry
	if req.ExpireAfterSeconds.present {
		if !req.ExpireAfterSeconds.valid || req.ExpireAfterSeconds.value == 0 {
			return nil, nil
		}
		secs := req.ExpireAfterSeconds.value
		if secs < 0 || secs > int64(maxExpiry/time.Second) {
			return nil, invalid("'expireAfterSeconds' must be between 0 and %d. Got %d", int64(maxExpiry/time.Second), secs)
		}
		after = time.Duration(secs) * time.Second
	}
	t := now.Add(after)
	return &t, nil
}

// formatPayee checks that phone is a Swish number or a Swedish mobile number
// and returns it in the national format, e.g. "070-123 45 67".
func formatPayee(phone string) (string, error) {
	if isSwishNumber(phone) {
		return phone, nil
	}
	num, err := phonenumbers.Parse(phone, "SE")
	if err != nil {
		return "", invalid("'%s' is not a valid phone number", phone)
	}
	if num.GetCountryCode() != swedenCountryCode || phonenumbers.GetNumberType(num) != phonenumbers.MOBILE {
		return "", invalid("'%s' is not a mobile number", phone)
	}
	return phonenumbers.Format(num, phonenumbers.NATIONAL), nil
}

// isSwishNumber reports whether phone looks like a business Swish number:
// ten digits starting with 123.
func isSwishNumber(phone string) bool {
	if len(phone) != swishNumberLength || !strings.HasPrefix(phone, swishNumberPrefix) {
		return false
	}
	return strings.Trim(phone, "0123456789") == ""
}
