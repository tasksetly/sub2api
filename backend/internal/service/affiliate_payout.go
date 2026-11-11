package service

import (
	"context"
	"encoding/hex"
	"math"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrAffiliatePayoutInvalid   = infraerrors.BadRequest("AFFILIATE_PAYOUT_INVALID", "invalid USDT payout: check network, address, transaction hash, exchange rate and images")
	ErrAffiliatePayoutDuplicate = infraerrors.BadRequest("AFFILIATE_PAYOUT_DUPLICATE", "this network transaction has already been recorded")
)

// AffiliatePayout 金额由服务端换算，截图存入凭证工单；历史流水不随汇率配置变动。
type AffiliatePayout struct {
	Network       string             `json:"network"`
	WalletAddress string             `json:"wallet_address"`
	TxHash        string             `json:"tx_hash"`
	ExchangeRate  float64            `json:"exchange_rate"`
	USDTAmount    float64            `json:"usdt_amount"`
	Note          string             `json:"note"`
	TicketID      int64              `json:"ticket_id"`
	AdminID       int64              `json:"-"`
	Attachments   []TicketAttachment `json:"-"`
}

func validAffiliateUSDTExchangeRate(rate float64) bool {
	return !math.IsNaN(rate) && !math.IsInf(rate, 0) && rate >= 0.00000001 && roundTo(rate, 8) < 1e12
}

func (s *AffiliateService) usdtExchangeRate(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateUSDTExchangeRateDefault
	}
	return s.settingService.GetAffiliateUSDTExchangeRate(ctx)
}

func normalizeAffiliatePayout(input AffiliatePayout, amount float64) (*AffiliatePayout, error) {
	payout := &AffiliatePayout{
		Network:       strings.ToLower(strings.TrimSpace(input.Network)),
		WalletAddress: strings.TrimSpace(input.WalletAddress),
		TxHash:        strings.ToLower(strings.TrimSpace(input.TxHash)),
		ExchangeRate:  roundTo(input.ExchangeRate, 8),
		Note:          strings.TrimSpace(input.Note),
	}
	if (payout.Network != "polygon" && payout.Network != "bsc") || !validAffiliateUSDTExchangeRate(input.ExchangeRate) || len([]rune(payout.Note)) > 2000 {
		return nil, ErrAffiliatePayoutInvalid
	}
	for value, size := range map[string]int{payout.WalletAddress: 20, payout.TxHash: 32} {
		if len(value) != 2+size*2 || !strings.HasPrefix(value, "0x") {
			return nil, ErrAffiliatePayoutInvalid
		}
		decoded, err := hex.DecodeString(value[2:])
		if err != nil || len(decoded) != size || strings.Trim(value[2:], "0") == "" {
			return nil, ErrAffiliatePayoutInvalid
		}
	}
	payout.USDTAmount = roundTo(roundTo(amount, 8)/payout.ExchangeRate, 6)
	if math.IsNaN(payout.USDTAmount) || math.IsInf(payout.USDTAmount, 0) || payout.USDTAmount <= 0 || payout.USDTAmount >= 1e12 {
		return nil, ErrAffiliatePayoutInvalid
	}
	return payout, nil
}

// AdminWithdrawUSDT 打款凭证、流水与余额扣减在仓储的同一事务中登记。
func (s *AffiliateService) AdminWithdrawUSDT(ctx context.Context, userID, adminID int64, amount float64, key string, input AffiliatePayout, uploads []TicketUpload) (*AffiliateWithdrawResult, error) {
	if s == nil || s.repo == nil || s.ticketService == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate payout service unavailable")
	}
	if userID <= 0 || adminID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	key, err := NormalizeIdempotencyKey(key)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrIdempotencyKeyRequired
	}
	amount = roundTo(amount, 8)
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || amount >= 1e12 {
		return nil, ErrAffiliateWithdrawAmountInvalid
	}
	payout, err := normalizeAffiliatePayout(input, amount)
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, ErrAffiliatePayoutInvalid
	}
	attachments, store, err := s.ticketService.uploadImages(ctx, uploads)
	if err != nil {
		return nil, err
	}
	payout.Attachments = attachments
	payout.AdminID = adminID
	result, err := s.repo.WithdrawQuota(ctx, userID, amount, affiliateWithdrawOperationID(key), payout)
	if (err == nil && result != nil && result.Replayed) || infraerrors.IsBadRequest(err) || infraerrors.IsConflict(err) {
		s.ticketService.deleteUploaded(context.WithoutCancel(ctx), store, attachments)
	}
	// shortcut: 数据库提交结果未知时保留截图，避免已提交凭证丢图；孤立对象需定期清理。
	return result, err
}

func (s *AffiliateService) ListUserAffiliateTransfers(ctx context.Context, userID int64, page, pageSize int) ([]AffiliateTransferRecord, int64, error) {
	if userID <= 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(AffiliateRecordFilter{UserID: userID, Page: page, PageSize: pageSize, SortDesc: true}))
}
