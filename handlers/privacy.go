package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
)

const paymentPrivacyReply = "Не отправляйте в чат номер карты, срок действия, CVV/CVC, PIN и коды из SMS. Платёжные реквизиты в этом чате не принимаются. Оплата возможна только на официальной защищённой странице EKT."

var paymentDataPattern = regexp.MustCompile(`(?i)(?:\d[ \t-]*){13,19}|\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b|(?:cvv|cvc|сvv|сvc|пин|pin|otp|смс|sms|код.{0,12}(?:банк|подтверж)|срок.{0,12}(?:карт|действ))\s*[:=-]?\s*\d{2,8}`)
var paymentTopicPattern = regexp.MustCompile(`(?i)(cvv|cvc|номер.{0,8}карт|card\s+number|реквизит.{0,12}карт|pin\s*код|код.{0,10}(?:sms|смс))`)
var emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
var phonePattern = regexp.MustCompile(`(?:\+\d[\d ()-]{8,}\d|\b[78][\d ()-]{9,}\d)`)

func privateData(text string) bool { return paymentDataPattern.MatchString(text) }

func redactContactData(text string) string {
	text = emailPattern.ReplaceAllString(text, "[email скрыт]")
	return phonePattern.ReplaceAllString(text, "[телефон скрыт]")
}

// The pinned SDK predates the store field. Inject store:false at the HTTP
// boundary without changing SDK versions or enabling response persistence.
type privateCompletionTransport struct{ base http.RoundTripper }

func (t privateCompletionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body.Close()
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		payload["store"] = json.RawMessage("false")
		data, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
	}
	return t.base.RoundTrip(r)
}
