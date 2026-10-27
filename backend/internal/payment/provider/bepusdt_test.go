package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestBepusdtCreatePaymentSignsNativeJSONRequest(t *testing.T) {
	const token = "test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != bepusdtAPIPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		var amount json.Number
		if err := json.Unmarshal(body["amount"], &amount); err != nil {
			t.Fatalf("amount should be a JSON number: %v", err)
		}
		if amount.String() != "12.5" {
			t.Errorf("amount = %q, want 12.5", amount)
		}
		params := map[string]string{
			"amount": "12.5", "fiat": "USD", "name": "Balance recharge",
			"notify_url": "https://merchant.example/api/v1/payment/webhook/bepusdt",
			"order_id": "order-123", "redirect_url": "https://merchant.example/payment/result",
			"timeout": "300", "trade_type": "usdt.trc20",
		}
		var signature string
		if err := json.Unmarshal(body["signature"], &signature); err != nil {
			t.Fatalf("decode signature: %v", err)
		}
		if signature != bepusdtSign(params, token) {
			t.Errorf("signature = %q, want %q", signature, bepusdtSign(params, token))
		}
		_, _ = io.WriteString(w, `{"status_code":200,"data":{"trade_id":"trade-9","payment_url":"/pay/trade-9"}}`)
	}))
	defer server.Close()

	provider, err := NewBepusdt("instance-1", map[string]string{
		"apiBase": server.URL + "/api/v1/", "token": token,
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/bepusdt",
		"tradeType": "usdt.trc20", "fiat": "usd", "timeout": "300",
	})
	if err != nil {
		t.Fatalf("NewBepusdt: %v", err)
	}
	result, err := provider.CreatePayment(context.Background(), testBepusdtCreateRequest())
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if result.TradeNo != "trade-9" || result.PayURL != server.URL+"/pay/trade-9" || result.Currency != "USD" {
		t.Fatalf("result = %#v", result)
	}
}

func TestBepusdtQueryOrderAndCancelPaymentUseNativeEndpoints(t *testing.T) {
	const token = "test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("request method = %s, want POST", r.Method)
		}
		switch r.URL.Path {
		case bepusdtInfoPath:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode query request: %v", err)
			}
			if body["trade_id"] != "trade-9" {
				t.Errorf("query trade_id = %q, want trade-9", body["trade_id"])
			}
			_, _ = io.WriteString(w, `{"status_code":200,"data":{"trade_id":"trade-9","order_id":"order-123","status":2,"money":"12.50","actual_amount":"1.234","fiat":"USD","block_transaction_id":"block-1"}}`)
		case bepusdtCancelPath:
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode cancel request: %v", err)
			}
			if body["trade_id"] != "trade-9" {
				t.Errorf("cancel trade_id = %q, want trade-9", body["trade_id"])
			}
			wantSignature := bepusdtSign(map[string]string{"trade_id": "trade-9"}, token)
			if body["signature"] != wantSignature {
				t.Errorf("cancel signature = %q, want %q", body["signature"], wantSignature)
			}
			_, _ = io.WriteString(w, `{"status_code":200,"data":{"trade_id":"trade-9"}}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider, err := NewBepusdt("instance-1", map[string]string{
		"apiBase": server.URL, "token": token, "notifyUrl": "https://merchant.example/notify",
	})
	if err != nil {
		t.Fatalf("NewBepusdt: %v", err)
	}
	queried, err := provider.QueryOrder(context.Background(), "trade-9")
	if err != nil {
		t.Fatalf("QueryOrder: %v", err)
	}
	if queried.Status != payment.ProviderStatusPaid || queried.TradeNo != "trade-9" || queried.Amount != 12.5 || queried.Metadata["currency"] != "USD" {
		t.Fatalf("QueryOrder = %#v", queried)
	}
	if err := provider.CancelPayment(context.Background(), "trade-9"); err != nil {
		t.Fatalf("CancelPayment: %v", err)
	}
}

func testBepusdtCreateRequest() payment.CreatePaymentRequest {
	return payment.CreatePaymentRequest{
		OrderID: "order-123", Amount: "12.50", PaymentType: "bepusdt",
		Subject: "Balance recharge", ReturnURL: "https://merchant.example/payment/result",
	}
}

func TestBepusdtVerifyNotification(t *testing.T) {
	provider, err := NewBepusdt("instance-1", map[string]string{
		"apiBase": "https://bepusdt.example", "token": "test-token",
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/bepusdt", "fiat": "CNY",
	})
	if err != nil {
		t.Fatalf("NewBepusdt: %v", err)
	}

	params := map[string]string{
		"actual_amount": "1.234", "amount": "12.50", "block_transaction_id": "block-1",
		"order_id": "order-123", "status": "2", "token": "usdt.trc20", "trade_id": "trade-9",
	}
	payload := makeBepusdtCallback(t, params, "test-token")
	notification, err := provider.VerifyNotification(context.Background(), payload, nil)
	if err != nil {
		t.Fatalf("VerifyNotification: %v", err)
	}
	if notification == nil || notification.OrderID != "order-123" || notification.TradeNo != "trade-9" || notification.Amount != 12.5 {
		t.Fatalf("notification = %#v", notification)
	}
	if notification.Metadata["currency"] != "CNY" || notification.Metadata["actual_amount"] != "1.234" {
		t.Fatalf("metadata = %#v", notification.Metadata)
	}

	tampered := strings.Replace(payload, `"amount":12.5`, `"amount":13.5`, 1)
	if _, err := provider.VerifyNotification(context.Background(), tampered, nil); err == nil {
		t.Fatal("expected amount tampering to fail signature verification")
	}
}

func TestBepusdtPendingNotificationIsIgnored(t *testing.T) {
	provider, err := NewBepusdt("instance-1", map[string]string{
		"apiBase": "https://bepusdt.example", "token": "test-token",
		"notifyUrl": "https://merchant.example/notify",
	})
	if err != nil {
		t.Fatalf("NewBepusdt: %v", err)
	}
	payload := makeBepusdtCallback(t, map[string]string{"order_id": "order-123", "status": "1"}, "test-token")
	notification, err := provider.VerifyNotification(context.Background(), payload, nil)
	if err != nil || notification != nil {
		t.Fatalf("VerifyNotification = (%#v, %v), want (nil, nil)", notification, err)
	}
}

func TestBepusdtNonPaidLifecycleNotificationsAreIgnored(t *testing.T) {
	provider, err := NewBepusdt("instance-1", map[string]string{
		"apiBase": "https://bepusdt.example", "token": "test-token",
		"notifyUrl": "https://merchant.example/notify",
	})
	if err != nil {
		t.Fatalf("NewBepusdt: %v", err)
	}
	for _, status := range []string{"3", "4", "5", "6"} {
		payload := makeBepusdtCallback(t, map[string]string{"order_id": "order-123", "status": status}, "test-token")
		notification, err := provider.VerifyNotification(context.Background(), payload, nil)
		if err != nil || notification != nil {
			t.Fatalf("status %s VerifyNotification = (%#v, %v), want (nil, nil)", status, notification, err)
		}
	}
}

func TestNewBepusdtRejectsInvalidConfiguration(t *testing.T) {
	base := map[string]string{
		"apiBase": "https://bepusdt.example", "token": "test-token", "notifyUrl": "https://merchant.example/notify",
	}
	for name, update := range map[string]func(map[string]string){
		"unsupported fiat": func(config map[string]string) { config["fiat"] = "CAD" },
		"short timeout":    func(config map[string]string) { config["timeout"] = "119" },
		"non-http API URL": func(config map[string]string) { config["apiBase"] = "ftp://bepusdt.example" },
	} {
		t.Run(name, func(t *testing.T) {
			config := make(map[string]string, len(base))
			for key, value := range base {
				config[key] = value
			}
			update(config)
			if _, err := NewBepusdt("instance-1", config); err == nil {
				t.Fatal("expected invalid configuration to be rejected")
			}
		})
	}
}

func makeBepusdtCallback(t *testing.T, params map[string]string, token string) string {
	t.Helper()
	signedParams := make(map[string]string, len(params))
	for key, value := range params {
		if key == "amount" || key == "actual_amount" {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatalf("parse callback %s: %v", key, err)
			}
			value = strconv.FormatFloat(parsed, 'g', -1, 64)
		}
		signedParams[key] = value
	}
	params["signature"] = bepusdtSign(signedParams, token)
	body := make(map[string]any, len(params))
	for key, value := range params {
		switch key {
		case "amount", "actual_amount", "status":
			if canonical, ok := signedParams[key]; ok {
				value = canonical
			}
			body[key] = json.Number(value)
		default:
			body[key] = value
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode callback: %v", err)
	}
	return string(encoded)
}
