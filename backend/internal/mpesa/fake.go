package mpesa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Fake is a stand-in for Daraja for local runs and tests (A-078). It
// answers OAuth, B2C and status requests the way Daraja documents them and
// posts the asynchronous results to the callback URLs it was given. The
// payee phone's last digit picks the outcome, so every path can be shown:
//
//	9  the payment fails (result code 2001)
//	8  Safaricom's queue times out; a status query then finds it paid
//	7  the request gets a 500 but was paid; a status query finds it
//	6  the request is refused synchronously (400 with an error body)
//	5  paid, but the number is registered in another name
//	   anything else is paid
//
// Its receipts start with "FK" so they can never be mistaken for real ones.
type Fake struct {
	// Deliver posts a callback. Nil uses http.Post.
	Deliver func(url string, body []byte) error
	// Delay before a callback is posted. Ignored when Manual.
	Delay time.Duration
	// Manual keeps callbacks until Flush (tests).
	Manual bool
	// NameFor gives the registered name for a phone (2547XXXXXXXX). Nil uses
	// SampleName.
	NameFor func(phone string) string

	mu       sync.Mutex
	n        int
	sent     map[string]int // payment OriginatorConversationID -> requests seen
	payments map[string]*fakePayment
	queue    []fakeCallback
}

type fakePayment struct {
	amount         int64
	phone          string
	conversationID string
	receipt        string
	paid           bool
	resultURL      string
}

type fakeCallback struct {
	url  string
	body []byte
}

// NewFake returns a fake Daraja.
func NewFake() *Fake {
	return &Fake{sent: map[string]int{}, payments: map[string]*fakePayment{}}
}

// Requests reports how many payment requests carried this
// OriginatorConversationID (tests prove none is sent twice).
func (f *Fake) Requests(originatorID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent[originatorID]
}

// TotalRequests is the number of payment requests received.
func (f *Fake) TotalRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, v := range f.sent {
		n += v
	}
	return n
}

// Flush posts every held callback and returns how many it posted.
func (f *Fake) Flush() int {
	f.mu.Lock()
	q := f.queue
	f.queue = nil
	f.mu.Unlock()
	for _, c := range q {
		f.post(c)
	}
	return len(q)
}

func (f *Fake) post(c fakeCallback) {
	if f.Deliver != nil {
		_ = f.Deliver(c.url, c.body)
		return
	}
	res, err := http.Post(c.url, "application/json", bytes.NewReader(c.body))
	if err == nil {
		res.Body.Close()
	}
}

func (f *Fake) schedule(url string, body any) {
	b, _ := json.Marshal(body)
	c := fakeCallback{url: url, body: b}
	if f.Manual {
		f.queue = append(f.queue, c)
		return
	}
	go func() {
		time.Sleep(f.Delay)
		f.post(c)
	}()
}

func (f *Fake) next() int { f.n++; return f.n }

// ServeHTTP implements the three Daraja endpoints MezAI uses.
func (f *Fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reply := func(status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/oauth/v1/generate":
		if _, _, ok := r.BasicAuth(); !ok {
			reply(http.StatusBadRequest, map[string]string{"errorCode": "400.008.01", "errorMessage": "Invalid Authentication passed"})
			return
		}
		reply(http.StatusOK, map[string]string{"access_token": "fake-token", "expires_in": "3599"})
		return
	case r.Header.Get("Authorization") != "Bearer fake-token":
		reply(http.StatusUnauthorized, map[string]string{"errorCode": "404.001.03", "errorMessage": "Invalid Access Token"})
		return
	case r.Method == http.MethodPost && r.URL.Path == "/mpesa/b2c/v3/paymentrequest":
		f.pay(r, reply)
	case r.Method == http.MethodPost && r.URL.Path == "/mpesa/transactionstatus/v1/query":
		f.query(r, reply)
	default:
		reply(http.StatusNotFound, map[string]string{"errorCode": "404.001.01", "errorMessage": "Resource not found"})
	}
}

func (f *Fake) pay(r *http.Request, reply func(int, any)) {
	var req struct {
		OriginatorConversationID string
		Amount                   json.Number
		PartyB                   string
		ResultURL                string
		QueueTimeOutURL          string
		InitiatorName            string
		SecurityCredential       string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OriginatorConversationID == "" ||
		req.ResultURL == "" || req.QueueTimeOutURL == "" || len(req.PartyB) != 12 {
		reply(http.StatusBadRequest, map[string]string{"errorCode": "400.002.02", "errorMessage": "Bad Request - Invalid request"})
		return
	}
	amount, err := req.Amount.Int64()
	if err != nil || amount <= 0 {
		reply(http.StatusBadRequest, map[string]string{"errorCode": "400.002.02", "errorMessage": "Bad Request - Invalid Amount"})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[req.OriginatorConversationID]++
	last := req.PartyB[len(req.PartyB)-1]
	if last == '6' {
		reply(http.StatusBadRequest, map[string]string{"errorCode": "400.002.02", "errorMessage": "Bad Request - Invalid PartyB (Fake Daraja)"})
		return
	}
	n := f.next()
	p := &fakePayment{amount: amount, phone: req.PartyB, conversationID: fmt.Sprintf("AG_FAKE_%06d", n), resultURL: req.ResultURL}
	f.payments[req.OriginatorConversationID] = p
	result := map[string]any{
		"ResultType": 0, "OriginatorConversationID": req.OriginatorConversationID, "ConversationID": p.conversationID,
	}
	switch last {
	case '9':
		result["ResultCode"] = 2001
		result["ResultDesc"] = "The initiator information is invalid. (Fake Daraja)"
		result["TransactionID"] = fmt.Sprintf("FKX%07d", n)
		f.schedule(req.ResultURL, map[string]any{"Result": result})
	default:
		p.paid = true
		p.receipt = fmt.Sprintf("FK%08d", n)
		result["ResultCode"] = 0
		result["ResultDesc"] = "The service request is processed successfully. (Fake Daraja)"
		result["TransactionID"] = p.receipt
		result["ResultParameters"] = map[string]any{"ResultParameter": []map[string]any{
			{"Key": "TransactionAmount", "Value": amount},
			{"Key": "TransactionReceipt", "Value": p.receipt},
			{"Key": "ReceiverPartyPublicName", "Value": req.PartyB + " - " + f.name(req.PartyB)},
			{"Key": "TransactionCompletedDateTime", "Value": time.Now().Format("02.01.2006 15:04:05")},
		}}
		switch last {
		case '8':
			f.schedule(req.QueueTimeOutURL, map[string]any{"Result": map[string]any{
				"ResultType": 1, "ResultCode": "SFC_IC0003", "ResultDesc": "Request timed out in the queue (Fake Daraja)",
				"OriginatorConversationID": req.OriginatorConversationID, "ConversationID": p.conversationID,
			}})
		case '7':
			reply(http.StatusInternalServerError, map[string]string{"errorCode": "500.003.02", "errorMessage": "System is busy (Fake Daraja)"})
			return
		default:
			f.schedule(req.ResultURL, map[string]any{"Result": result})
		}
	}
	reply(http.StatusOK, map[string]string{
		"ConversationID": p.conversationID, "OriginatorConversationID": req.OriginatorConversationID,
		"ResponseCode": "0", "ResponseDescription": "Accept the service request successfully.",
	})
}

func (f *Fake) query(r *http.Request, reply func(int, any)) {
	var req struct {
		OriginatorConversationID string
		OriginalConversationID   string
		ResultURL                string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OriginatorConversationID == "" || req.ResultURL == "" {
		reply(http.StatusBadRequest, map[string]string{"errorCode": "400.002.02", "errorMessage": "Bad Request - Invalid request"})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	n := f.next()
	conv := fmt.Sprintf("AG_FAKEQ_%06d", n)
	result := map[string]any{"ResultType": 0, "OriginatorConversationID": req.OriginatorConversationID, "ConversationID": conv}
	p := f.payments[strings.TrimSpace(req.OriginalConversationID)]
	switch {
	case p == nil:
		result["ResultCode"] = 2032
		result["ResultDesc"] = "The transaction could not be found. (Fake Daraja)"
	case p.paid:
		result["ResultCode"] = 0
		result["ResultDesc"] = "The service request is processed successfully. (Fake Daraja)"
		result["ResultParameters"] = map[string]any{"ResultParameter": []map[string]any{
			{"Key": "ReceiptNo", "Value": p.receipt},
			{"Key": "CreditPartyName", "Value": p.phone + " - " + f.name(p.phone)},
			{"Key": "TransactionStatus", "Value": "Completed"},
			{"Key": "Amount", "Value": p.amount},
		}}
	default:
		result["ResultCode"] = 0
		result["ResultDesc"] = "The service request is processed successfully. (Fake Daraja)"
		result["ResultParameters"] = map[string]any{"ResultParameter": []map[string]any{
			{"Key": "TransactionStatus", "Value": "Failed"},
			{"Key": "Amount", "Value": p.amount},
		}}
	}
	f.schedule(req.ResultURL, map[string]any{"Result": result})
	reply(http.StatusOK, map[string]string{
		"ConversationID": conv, "OriginatorConversationID": req.OriginatorConversationID,
		"ResponseCode": "0", "ResponseDescription": "Accept the service request successfully.",
	})
}

func (f *Fake) name(phone string) string {
	if f.NameFor != nil {
		return f.NameFor(phone)
	}
	return SampleName(phone)
}

// SampleName is the fake's registered name for a phone. The development
// seed gives "Sample Contributor 0NN" the number +254 700 000 0NN, so those
// numbers get that name back and compare as a match; a number ending in 5
// is registered to somebody else, to show a mismatch.
func SampleName(phone string) string {
	if strings.HasSuffix(phone, "5") {
		return "SAMPLE DIFFERENT PERSON"
	}
	if len(phone) == 12 && strings.HasPrefix(phone, "2547000000") {
		return "SAMPLE CONTRIBUTOR 0" + phone[10:]
	}
	return "SAMPLE PAYEE"
}
