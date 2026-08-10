package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayEpusdtRequiresCanonicalNumericPID(t *testing.T) {
	t.Parallel()

	_, err := NewEasyPay("epusdt-instance", map[string]string{
		"pid":       "merchant-1000",
		"pkey":      "epusdt-secret",
		"apiBase":   "https://epusdt.example/payments/epay/v1/order/create-transaction",
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/easypay",
		"returnUrl": "https://merchant.example/payment/result",
	})
	if err == nil || !strings.Contains(err.Error(), "canonical numeric pid") {
		t.Fatalf("NewEasyPay error = %v, want canonical numeric PID error", err)
	}
}

func TestEasyPayEpusdtUsesRedirectCheckout(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("epusdt-instance", map[string]string{
		"pid":         "1000",
		"pkey":        "epusdt-secret",
		"apiBase":     "https://epusdt.example/payments/epay/v1/order/create-transaction/submit.php",
		"notifyUrl":   "https://merchant.example/api/v1/payment/webhook/easypay",
		"returnUrl":   "https://merchant.example/payment/result",
		"paymentMode": "qrcode",
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	response, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2_epusdt_1",
		Amount:      "12.50",
		PaymentType: payment.TypeAlipay,
		Subject:     "Sub2API balance recharge",
	})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if response.PaymentMode != paymentModeRedirect {
		t.Fatalf("payment mode = %q, want %q", response.PaymentMode, paymentModeRedirect)
	}
	if !response.DirectRedirect {
		t.Fatal("direct redirect = false, want true for Epusdt checkout")
	}
	if response.QRCode != "" {
		t.Fatalf("qrcode = %q, want empty for Epusdt redirect checkout", response.QRCode)
	}

	payURL, err := url.Parse(response.PayURL)
	if err != nil {
		t.Fatalf("parse pay URL: %v", err)
	}
	if payURL.Path != epusdtEPayBasePath+"/submit.php" {
		t.Fatalf("pay URL path = %q, want %q", payURL.Path, epusdtEPayBasePath+"/submit.php")
	}
	params := make(map[string]string)
	for key, values := range payURL.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	for key, want := range map[string]string{
		"pid":          "1000",
		"type":         "alipay",
		"out_trade_no": "sub2_epusdt_1",
		"money":        "12.50",
		"notify_url":   "https://merchant.example/api/v1/payment/webhook/easypay",
	} {
		if got := params[key]; got != want {
			t.Fatalf("param[%s] = %q, want %q", key, got, want)
		}
	}
	if !easyPayVerifySign(params, "epusdt-secret", params["sign"]) {
		t.Fatalf("Epusdt redirect URL has an invalid EasyPay signature: %s", response.PayURL)
	}
}

func TestEasyPayEpusdtRejectsWxpayType(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("epusdt-instance", map[string]string{
		"pid":       "1000",
		"pkey":      "epusdt-secret",
		"apiBase":   "https://epusdt.example/payments/epay/v1/order/create-transaction",
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/easypay",
		"returnUrl": "https://merchant.example/payment/result",
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	_, err = provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2_epusdt_wxpay",
		Amount:      "12.50",
		PaymentType: payment.TypeWxpay,
		Subject:     "Unsupported WeChat payment",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support wxpay") {
		t.Fatalf("CreatePayment error = %v, want Epusdt wxpay error", err)
	}
}

func TestEasyPayVerifiesEpusdtEPayCallback(t *testing.T) {
	t.Parallel()

	provider, err := NewEasyPay("epusdt-instance", map[string]string{
		"pid":       "1000",
		"pkey":      "epusdt-secret",
		"apiBase":   "https://epusdt.example/payments/epay/v1/order/create-transaction",
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/easypay",
		"returnUrl": "https://merchant.example/payment/result",
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	params := map[string]string{
		"pid":          "1000",
		"trade_no":     "20260523171652123456001",
		"out_trade_no": "sub2_epusdt_callback",
		"type":         "alipay",
		"name":         "Sub2API balance recharge",
		"money":        "12.5000",
		"trade_status": "TRADE_SUCCESS",
	}
	params["sign"] = easyPaySign(params, "epusdt-secret")
	params["sign_type"] = "MD5"
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}

	notification, err := provider.VerifyNotification(context.Background(), values.Encode(), nil)
	if err != nil {
		t.Fatalf("VerifyNotification: %v", err)
	}
	if notification.Status != payment.ProviderStatusSuccess {
		t.Fatalf("notification status = %q, want %q", notification.Status, payment.ProviderStatusSuccess)
	}
	if notification.OrderID != "sub2_epusdt_callback" || notification.TradeNo != "20260523171652123456001" {
		t.Fatalf("notification identifiers = %+v", notification)
	}
	if notification.Amount != 12.5 || notification.Metadata["pid"] != "1000" {
		t.Fatalf("notification amount/metadata = %+v", notification)
	}
}

func TestEasyPayEpusdtSkipsUnsupportedQueryAndRefundEndpoints(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests++
	}))
	defer server.Close()

	provider, err := NewEasyPay("epusdt-instance", map[string]string{
		"pid":       "1000",
		"pkey":      "epusdt-secret",
		"apiBase":   server.URL + epusdtEPayBasePath,
		"notifyUrl": "https://merchant.example/api/v1/payment/webhook/easypay",
		"returnUrl": "https://merchant.example/payment/result",
	})
	if err != nil {
		t.Fatalf("NewEasyPay: %v", err)
	}

	if _, err := provider.QueryOrder(context.Background(), "sub2_epusdt_2"); err == nil || !strings.Contains(err.Error(), "does not support order queries") {
		t.Fatalf("QueryOrder error = %v, want Epusdt unsupported-query error", err)
	}
	if _, err := provider.Refund(context.Background(), payment.RefundRequest{OrderID: "sub2_epusdt_2", Amount: "12.50"}); err == nil || !strings.Contains(err.Error(), "does not support refunds") {
		t.Fatalf("Refund error = %v, want Epusdt unsupported-refund error", err)
	}
	if requests != 0 {
		t.Fatalf("unexpected upstream requests = %d", requests)
	}
}
