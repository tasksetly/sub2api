package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

const (
	bepusdtHTTPTimeout     = 15 * time.Second
	maxBepusdtResponseSize = 1 << 20
	defaultBepusdtFiat     = "CNY"
	defaultBepusdtTrade    = "usdt.trc20"
	bepusdtAPIPath         = "/api/v1/order/create-transaction"
	bepusdtInfoPath        = "/api/v1/pay/info"
	bepusdtCancelPath      = "/api/v1/order/cancel-transaction"
)

var bepUsdtFiatCurrencies = map[string]struct{}{
	"CNY": {}, "USD": {}, "EUR": {}, "GBP": {}, "JPY": {},
}

// Bepusdt implements the native BEpusdt JSON API and notification protocol.
type Bepusdt struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

// NewBepusdt validates and copies a provider instance's configuration.
// Required config keys: apiBase, token, notifyUrl. Optional: tradeType, fiat, timeout.
func NewBepusdt(instanceID string, config map[string]string) (*Bepusdt, error) {
	for _, key := range []string{"apiBase", "token", "notifyUrl"} {
		if strings.TrimSpace(config[key]) == "" {
			return nil, fmt.Errorf("bepusdt config missing required key: %s", key)
		}
	}
	base, err := normalizeBepusdtAPIBase(config["apiBase"])
	if err != nil {
		return nil, err
	}
	cfg := make(map[string]string, len(config))
	for key, value := range config {
		cfg[key] = value
	}
	cfg["apiBase"] = base
	cfg["notifyUrl"] = strings.TrimSpace(cfg["notifyUrl"])
	cfg["token"] = strings.TrimSpace(cfg["token"])
	cfg["tradeType"] = strings.TrimSpace(cfg["tradeType"])
	if cfg["tradeType"] == "" {
		cfg["tradeType"] = defaultBepusdtTrade
	}
	cfg["fiat"] = strings.ToUpper(strings.TrimSpace(cfg["fiat"]))
	if cfg["fiat"] == "" {
		cfg["fiat"] = defaultBepusdtFiat
	}
	if _, ok := bepUsdtFiatCurrencies[cfg["fiat"]]; !ok {
		return nil, fmt.Errorf("bepusdt fiat must be one of CNY, USD, EUR, GBP, JPY")
	}
	if timeout := strings.TrimSpace(cfg["timeout"]); timeout != "" {
		seconds, err := strconv.Atoi(timeout)
		if err != nil || seconds < 120 {
			return nil, fmt.Errorf("bepusdt timeout must be an integer of at least 120 seconds")
		}
		cfg["timeout"] = strconv.Itoa(seconds)
	}
	return &Bepusdt{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: bepusdtHTTPTimeout},
	}, nil
}

func normalizeBepusdtAPIBase(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("bepusdt apiBase must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	parsed.RawPath = ""
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(strings.ToLower(path), "/api/v1") {
		path = path[:len(path)-len("/api/v1")]
	}
	parsed.Path = strings.TrimRight(path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (b *Bepusdt) Name() string        { return "BEpusdt" }
func (b *Bepusdt) ProviderKey() string { return payment.TypeBepusdt }
func (b *Bepusdt) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeBepusdt}
}

func (b *Bepusdt) MerchantIdentityMetadata() map[string]string {
	if b == nil {
		return nil
	}
	return map[string]string{"currency": b.config["fiat"]}
}

func (b *Bepusdt) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	amount := strings.TrimSpace(req.Amount)
	parsedAmount, err := strconv.ParseFloat(amount, 64)
	if err != nil || parsedAmount <= 0 || math.IsInf(parsedAmount, 0) || math.IsNaN(parsedAmount) {
		return nil, fmt.Errorf("bepusdt amount must be a positive number")
	}
	// BEpusdt parses JSON numbers into float64 before calculating its
	// signature with fmt.Sprintf("%v", value). Canonicalize the amount the
	// same way so an input such as "12.50" is signed as "12.5".
	amount = strconv.FormatFloat(parsedAmount, 'g', -1, 64)
	redirectURL := strings.TrimSpace(req.ReturnURL)
	if redirectURL == "" {
		return nil, fmt.Errorf("bepusdt return URL is required")
	}
	name := strings.TrimSpace(req.Subject)
	if name == "" {
		name = "Sub2API order"
	}

	params := map[string]string{
		"order_id":    strings.TrimSpace(req.OrderID),
		"amount":      amount,
		"notify_url":  b.config["notifyUrl"],
		"redirect_url": redirectURL,
		"trade_type":  b.config["tradeType"],
		"fiat":        b.config["fiat"],
		"name":        name,
	}
	if strings.TrimSpace(req.OrderID) == "" {
		return nil, fmt.Errorf("bepusdt order ID is required")
	}
	if timeout := b.config["timeout"]; timeout != "" {
		params["timeout"] = timeout
	}
	params["signature"] = bepusdtSign(params, b.config["token"])

	requestBody := make(map[string]any, len(params))
	for key, value := range params {
		if key == "amount" {
			requestBody[key] = json.Number(value)
		} else if key == "timeout" {
			seconds, _ := strconv.Atoi(value)
			requestBody[key] = seconds
		} else {
			requestBody[key] = value
		}
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("bepusdt encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+bepusdtAPIPath, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bepusdt create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bepusdt create request failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBepusdtResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("bepusdt read response: %w", err)
	}
	if len(responseBody) > maxBepusdtResponseSize {
		return nil, fmt.Errorf("bepusdt response exceeded size limit")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("bepusdt returned HTTP status %d", resp.StatusCode)
	}
	var result struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Data       struct {
			TradeID   string `json:"trade_id"`
			PaymentURL string `json:"payment_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, fmt.Errorf("bepusdt decode response: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt rejected payment creation (status_code %d)", result.StatusCode)
	}
	payURL, err := resolveBepusdtPaymentURL(b.config["apiBase"], result.Data.PaymentURL)
	if err != nil {
		return nil, err
	}
	tradeID := strings.TrimSpace(result.Data.TradeID)
	if tradeID == "" || payURL == "" {
		return nil, fmt.Errorf("bepusdt response missing trade_id or payment_url")
	}
	return &payment.CreatePaymentResponse{TradeNo: tradeID, PayURL: payURL, Currency: b.config["fiat"]}, nil
}

func resolveBepusdtPaymentURL(apiBase, raw string) (string, error) {
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("bepusdt response contains an invalid payment URL")
	}
	base, err := url.Parse(apiBase)
	if err != nil {
		return "", fmt.Errorf("bepusdt apiBase is invalid")
	}
	if !ref.IsAbs() {
		ref = base.ResolveReference(ref)
	}
	if (ref.Scheme != "http" && ref.Scheme != "https") || ref.Host == "" || ref.User != nil {
		return "", fmt.Errorf("bepusdt response payment URL must use HTTP(S)")
	}
	return ref.String(), nil
}

func (b *Bepusdt) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, errors.New("bepusdt trade ID is required for order query")
	}
	body, err := json.Marshal(map[string]string{"trade_id": tradeNo})
	if err != nil {
		return nil, fmt.Errorf("bepusdt encode order query: %w", err)
	}
	responseBody, err := b.postJSON(ctx, bepusdtInfoPath, body)
	if err != nil {
		return nil, fmt.Errorf("bepusdt query order: %w", err)
	}
	var result struct {
		StatusCode int            `json:"status_code"`
		Message    string         `json:"message"`
		Data       map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("bepusdt decode order query: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bepusdt order query rejected (status_code %d): %s", result.StatusCode, strings.TrimSpace(result.Message))
	}
	statusText := strings.TrimSpace(bepusdtMapScalar(result.Data, "status"))
	status, err := strconv.Atoi(statusText)
	if err != nil {
		return nil, fmt.Errorf("bepusdt order query returned invalid status")
	}
	amountText := strings.TrimSpace(bepusdtMapScalar(result.Data, "money"))
	if amountText == "" {
		amountText = strings.TrimSpace(bepusdtMapScalar(result.Data, "amount"))
	}
	amount := 0.0
	if amountText != "" {
		amount, err = strconv.ParseFloat(amountText, 64)
	}
	if status == 2 && (amountText == "" || err != nil || amount <= 0 || math.IsInf(amount, 0) || math.IsNaN(amount)) {
		return nil, fmt.Errorf("bepusdt paid order query returned invalid amount")
	}
	if err != nil || amount < 0 || math.IsInf(amount, 0) || math.IsNaN(amount) {
		return nil, fmt.Errorf("bepusdt order query returned invalid amount")
	}
	currency := strings.ToUpper(strings.TrimSpace(bepusdtMapScalar(result.Data, "fiat")))
	if currency == "" {
		currency = b.config["fiat"]
	}
	metadata := map[string]string{"currency": currency, "status": statusText}
	for _, key := range []string{"actual_amount", "token", "block_transaction_id", "trade_url"} {
		if value := strings.TrimSpace(bepusdtMapScalar(result.Data, key)); value != "" {
			metadata[key] = value
		}
	}
	queryTradeID := strings.TrimSpace(bepusdtMapScalar(result.Data, "trade_id"))
	if queryTradeID == "" {
		queryTradeID = tradeNo
	}
	providerStatus := payment.ProviderStatusPending
	switch status {
	case 2:
		providerStatus = payment.ProviderStatusPaid
	case 3, 4, 6:
		providerStatus = payment.ProviderStatusFailed
	}
	return &payment.QueryOrderResponse{TradeNo: queryTradeID, Status: providerStatus, Amount: amount, Metadata: metadata}, nil
}

func (b *Bepusdt) CancelPayment(ctx context.Context, tradeNo string) error {
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return errors.New("bepusdt trade ID is required for cancellation")
	}
	params := map[string]string{"trade_id": tradeNo}
	params["signature"] = bepusdtSign(params, b.config["token"])
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("bepusdt encode cancellation: %w", err)
	}
	responseBody, err := b.postJSON(ctx, bepusdtCancelPath, body)
	if err != nil {
		return fmt.Errorf("bepusdt cancel order: %w", err)
	}
	var result struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(responseBody), &result); err != nil {
		return fmt.Errorf("bepusdt decode cancellation: %w", err)
	}
	if result.StatusCode != http.StatusOK {
		return fmt.Errorf("bepusdt cancellation rejected (status_code %d): %s", result.StatusCode, strings.TrimSpace(result.Message))
	}
	return nil
}

func (b *Bepusdt) postJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config["apiBase"]+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBepusdtResponseSize+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxBepusdtResponseSize {
		return nil, fmt.Errorf("response exceeded size limit")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("returned HTTP status %d", resp.StatusCode)
	}
	return responseBody, nil
}

func bepusdtMapScalar(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, _ := bepUsdtScalarString(value)
	return text
}

func (b *Bepusdt) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	decoder := json.NewDecoder(strings.NewReader(rawBody))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("bepusdt notification is not valid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("bepusdt notification contains trailing data")
	}
	if raw == nil {
		return nil, fmt.Errorf("bepusdt notification must be a JSON object")
	}
	params := make(map[string]string, len(raw))
	for key, value := range raw {
		text, err := bepUsdtScalarString(value)
		if err != nil {
			return nil, fmt.Errorf("bepusdt notification field %q must be a scalar", key)
		}
		params[key] = text
	}
	signature := strings.TrimSpace(params["signature"])
	if signature == "" || !hmac.Equal([]byte(strings.ToLower(signature)), []byte(bepusdtSign(params, b.config["token"]))) {
		return nil, fmt.Errorf("bepusdt notification signature mismatch")
	}
	status, err := strconv.Atoi(strings.TrimSpace(params["status"]))
	if err != nil {
		return nil, fmt.Errorf("bepusdt notification status is invalid")
	}
	// BEpusdt documents 1/3 as waiting/expired and newer releases also emit
	// 4/5/6 for cancelled/confirming/failed states. None of these statuses
	// represent a settled payment, but they should still be acknowledged so
	// the gateway does not retry them as failed notifications.
	if status == 1 || (status >= 3 && status <= 6) {
		return nil, nil
	}
	if status != 2 {
		return nil, fmt.Errorf("bepusdt notification has unsupported status %d", status)
	}
	orderID := strings.TrimSpace(params["order_id"])
	tradeID := strings.TrimSpace(params["trade_id"])
	amount, err := strconv.ParseFloat(strings.TrimSpace(params["amount"]), 64)
	if orderID == "" || tradeID == "" || err != nil || amount <= 0 || math.IsInf(amount, 0) || math.IsNaN(amount) {
		return nil, fmt.Errorf("bepusdt paid notification is missing a valid order, trade, or amount")
	}
	metadata := map[string]string{
		"currency": b.config["fiat"],
		"status":   strconv.Itoa(status),
	}
	for _, key := range []string{"actual_amount", "token", "block_transaction_id"} {
		if value := strings.TrimSpace(params[key]); value != "" {
			metadata[key] = value
		}
	}
	return &payment.PaymentNotification{
		TradeNo: tradeID,
		OrderID: orderID,
		Amount:  amount,
		Status:  payment.NotificationStatusSuccess,
		RawData: rawBody,
		Metadata: metadata,
	}, nil
}

func bepUsdtScalarString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case json.Number:
		number, err := v.Float64()
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
			return "", fmt.Errorf("invalid JSON number")
		}
		return strconv.FormatFloat(number, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("not a scalar")
	}
}

func bepusdtSign(params map[string]string, token string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key != "signature" && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	sum := md5.Sum([]byte(strings.Join(parts, "&") + token))
	return hex.EncodeToString(sum[:])
}

func (b *Bepusdt) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, errors.New("bepusdt does not document a refund API")
}

var _ payment.Provider = (*Bepusdt)(nil)
var _ payment.MerchantIdentityProvider = (*Bepusdt)(nil)
var _ payment.CancelableProvider = (*Bepusdt)(nil)
