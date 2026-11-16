package mpesa

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Removed replaces a value taken out of a callback before it is stored.
const Removed = "[removed]"

// nameKeys are result parameters that carry a person's or organisation's
// name, with a phone number or shortcode in front ("2547… - NAME").
var nameKeys = map[string]bool{"ReceiverPartyPublicName": true, "CreditPartyName": true, "DebitPartyName": true}

// Redact returns the callback body as it may be stored, and the payee's
// registered name as M-Pesa sent it (empty when absent). It removes the
// name parameters (which also hold the full phone number) and
// ReferenceData (which echoes our callback URL and its secret token). A
// body it cannot read is returned unchanged with no name.
func Redact(body []byte) (stored []byte, payeeName string) {
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return body, ""
	}
	result, ok := doc["Result"].(map[string]any)
	if !ok {
		return body, ""
	}
	delete(result, "ReferenceData")
	if rp, ok := result["ResultParameters"].(map[string]any); ok {
		var list []any
		switch v := rp["ResultParameter"].(type) {
		case []any:
			list = v
		case map[string]any:
			list = []any{v}
		}
		for _, item := range list {
			p, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := p["Key"].(string)
			if !nameKeys[key] {
				continue
			}
			if key != "DebitPartyName" && payeeName == "" && p["Value"] != nil {
				payeeName = fmt.Sprint(p["Value"])
			}
			p["Value"] = Removed
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return body, ""
	}
	return out, payeeName
}
