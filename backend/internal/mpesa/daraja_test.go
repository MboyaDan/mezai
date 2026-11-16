package mpesa

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string // ok, rejected, notaccepted, unknown
	}{
		{"accepted", 200, `{"ConversationID":"AG_1","OriginatorConversationID":"x","ResponseCode":"0","ResponseDescription":"Accept"}`, "ok"},
		{"refused in body", 200, `{"ResponseCode":"1","ResponseDescription":"Rejected"}`, "rejected"},
		{"accepted without conversation", 200, `{"ResponseCode":"0"}`, "unknown"},
		{"garbled 200", 200, `<html>`, "unknown"},
		{"bad request", 400, `{"errorCode":"400.002.02","errorMessage":"Invalid PartyB"}`, "rejected"},
		{"proxy 400 without error body", 400, `<html>bad gateway</html>`, "unknown"},
		{"expired token", 401, `{"errorCode":"404.001.03"}`, "notaccepted"},
		{"rate limited", 429, ``, "notaccepted"},
		{"server error", 500, `{"errorCode":"500.003.02","errorMessage":"busy"}`, "unknown"},
		{"gateway timeout", 504, ``, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := classify(c.status, []byte(c.body), nil)
			var rej *RejectedError
			got := "ok"
			switch {
			case errors.As(err, &rej):
				got = "rejected"
			case errors.Is(err, ErrNotAccepted):
				got = "notaccepted"
			case errors.Is(err, ErrUnknown):
				got = "unknown"
			case err != nil:
				got = err.Error()
			}
			if got != c.want {
				t.Fatalf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestParseResult(t *testing.T) {
	body := `{"Result":{"ResultType":0,"ResultCode":0,"ResultDesc":"The service request is processed successfully.",
	  "OriginatorConversationID":"mz-abc","ConversationID":"AG_20261116_1","TransactionID":"NLJ41HAY6Q",
	  "ResultParameters":{"ResultParameter":[{"Key":"TransactionAmount","Value":1200},
	  {"Key":"TransactionReceipt","Value":"NLJ41HAY6Q"},{"Key":"B2CUtilityAccountAvailableFunds","Value":10116.00}]},
	  "ReferenceData":{"ReferenceItem":{"Key":"QueueTimeoutURL","Value":"https://example.invalid"}}}}`
	r, err := ParseResult([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if r.ResultCode != "0" || r.OriginatorConversationID != "mz-abc" || r.TransactionID != "NLJ41HAY6Q" ||
		r.Params["TransactionAmount"] != "1200" || r.Params["B2CUtilityAccountAvailableFunds"] != "10116.00" {
		t.Fatalf("parsed: %+v", r)
	}
	// String result codes and a single parameter object.
	r, err = ParseResult([]byte(`{"Result":{"ResultType":0,"ResultCode":"2001","ResultDesc":"x","OriginatorConversationID":"a",
	  "ResultParameters":{"ResultParameter":{"Key":"TransactionStatus","Value":"Failed"}}}}`))
	if err != nil || r.ResultCode != "2001" || r.Params["TransactionStatus"] != "Failed" {
		t.Fatalf("parsed: %+v %v", r, err)
	}
	if _, err := ParseResult([]byte(`{"Body":{}}`)); err == nil {
		t.Fatal("a body without Result must be refused")
	}
}

func TestDarajaAgainstFake(t *testing.T) {
	fake := NewFake()
	fake.Manual = true
	srv := httptest.NewServer(fake)
	defer srv.Close()
	var delivered []string
	fake.Deliver = func(url string, body []byte) error { delivered = append(delivered, url+" "+string(body)); return nil }
	d := NewDaraja(Config{BaseURL: srv.URL, ConsumerKey: "k", ConsumerSecret: "s", ShortCode: "600000",
		InitiatorName: "i", SecurityCredential: "c", CallbackBase: "https://api.example.invalid/v1/mpesa/callbacks/tok"}, nil)
	ctx := context.Background()

	ack, err := d.PayB2C(ctx, Payment{OriginatorConversationID: "mz-line-1", AmountKES: 1200, Phone: "254700000001"})
	if err != nil || ack.ConversationID == "" {
		t.Fatalf("pay: %+v %v", ack, err)
	}
	if fake.Flush() != 1 || !strings.Contains(delivered[0], "/b2c/result") || !strings.Contains(delivered[0], `"ResultCode":0`) {
		t.Fatalf("callbacks: %v", delivered)
	}
	if _, err := d.PayB2C(ctx, Payment{OriginatorConversationID: "mz-line-6", AmountKES: 100, Phone: "254700000006"}); !isRejected(err) {
		t.Fatalf("phone ending 6 must be refused: %v", err)
	}
	if _, err := d.PayB2C(ctx, Payment{OriginatorConversationID: "mz-line-7", AmountKES: 100, Phone: "254700000007"}); !errors.Is(err, ErrUnknown) {
		t.Fatalf("phone ending 7 must be unknown: %v", err)
	}
	if _, err := d.QueryStatus(ctx, Query{OriginatorConversationID: "mz-line-7-q1", PaymentOriginatorID: "mz-line-7"}); err != nil {
		t.Fatal(err)
	}
	delivered = nil
	fake.Flush()
	if len(delivered) != 1 || !strings.Contains(delivered[0], "/status/result") || !strings.Contains(delivered[0], "Completed") {
		t.Fatalf("status callback: %v", delivered)
	}
	if fake.Requests("mz-line-1") != 1 {
		t.Fatal("request count")
	}
}

func TestTokenCachedAndRefreshedAfter401(t *testing.T) {
	var tokens, calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/v1/generate" {
			tokens.Add(1)
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":"3599"}`))
			return
		}
		if calls.Add(1) == 2 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"ConversationID":"AG","ResponseCode":"0"}`))
	}))
	defer srv.Close()
	d := NewDaraja(Config{BaseURL: srv.URL}, nil)
	ctx := context.Background()
	if _, err := d.PayB2C(ctx, Payment{}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PayB2C(ctx, Payment{}); !errors.Is(err, ErrNotAccepted) {
		t.Fatalf("401: %v", err)
	}
	if _, err := d.PayB2C(ctx, Payment{}); err != nil {
		t.Fatal(err)
	}
	if tokens.Load() != 2 {
		t.Fatalf("tokens fetched: %d (want one, then one more after the 401)", tokens.Load())
	}
}

func isRejected(err error) bool {
	var r *RejectedError
	return errors.As(err, &r)
}

func TestRedact(t *testing.T) {
	body := `{"Result":{"ResultType":0,"ResultCode":0,"OriginatorConversationID":"mz-a","ConversationID":"AG_1","TransactionID":"NLJ41HAY6Q",
	  "ResultParameters":{"ResultParameter":[{"Key":"TransactionAmount","Value":1200},
	  {"Key":"ReceiverPartyPublicName","Value":"254712345678 - JANE DOE"},{"Key":"B2CUtilityAccountAvailableFunds","Value":10116.00}]},
	  "ReferenceData":{"ReferenceItem":{"Key":"QueueTimeoutURL","Value":"https://api.example.invalid/v1/mpesa/callbacks/secret-token/b2c/timeout"}}}}`
	stored, name := Redact([]byte(body))
	if name != "254712345678 - JANE DOE" {
		t.Fatalf("name: %q", name)
	}
	for _, leaked := range []string{"JANE", "254712345678", "secret-token", "ReferenceData"} {
		if strings.Contains(string(stored), leaked) {
			t.Fatalf("%q is still in the stored body: %s", leaked, stored)
		}
	}
	// What processing needs is untouched, numbers included.
	r, err := ParseResult(stored)
	if err != nil || r.ResultCode != "0" || r.TransactionID != "NLJ41HAY6Q" || r.Params["TransactionAmount"] != "1200" ||
		r.Params["B2CUtilityAccountAvailableFunds"] != "10116.00" || r.Params["ReceiverPartyPublicName"] != Removed {
		t.Fatalf("after redaction: %+v %v", r, err)
	}
	// A status result names the payee as the credit party; our own name is removed but not returned.
	stored, name = Redact([]byte(`{"Result":{"ResultCode":0,"ResultParameters":{"ResultParameter":[
	  {"Key":"DebitPartyName","Value":"600000 - Sample Org"},{"Key":"CreditPartyName","Value":"254700000001 - JOHN DOE"}]}}}`))
	if name != "254700000001 - JOHN DOE" || strings.Contains(string(stored), "JOHN") || strings.Contains(string(stored), "Sample Org") {
		t.Fatalf("status result: %q %s", name, stored)
	}
	if stored, name := Redact([]byte(`not json`)); string(stored) != "not json" || name != "" {
		t.Fatal("an unreadable body must come back unchanged")
	}
}
