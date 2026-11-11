//go:build unit

package service

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func validTestAffiliatePayout() AffiliatePayout {
	return AffiliatePayout{Network: "polygon", WalletAddress: "0x" + strings.Repeat("1", 40), TxHash: "0x" + strings.Repeat("a", 64), ExchangeRate: 6.8}
}

func TestNormalizeAffiliatePayout_ConvertsCNYAndDiscardsClientReceipt(t *testing.T) {
	for _, network := range []string{"polygon", "bsc"} {
		input := validTestAffiliatePayout()
		input.Network = " " + strings.ToUpper(network) + " "
		input.TxHash = "0x" + strings.Repeat("A", 64)
		input.USDTAmount, input.TicketID, input.AdminID = 999, 999, 999
		input.Note = " paid "
		got, err := normalizeAffiliatePayout(input, 68)
		require.NoError(t, err)
		require.Equal(t, network, got.Network)
		require.Equal(t, 10.0, got.USDTAmount)
		require.Equal(t, "0x"+strings.Repeat("a", 64), got.TxHash)
		require.Equal(t, "paid", got.Note)
		require.Zero(t, got.TicketID)
		require.Zero(t, got.AdminID)
	}
	got, err := normalizeAffiliatePayout(validTestAffiliatePayout(), 10)
	require.NoError(t, err)
	require.Equal(t, 1.470588, got.USDTAmount)
}

func TestNormalizeAffiliatePayout_RejectsInvalidMoneyAndChainDetails(t *testing.T) {
	for _, rate := range []float64{0, -1, 1e-9, 1e12, math.NaN(), math.Inf(1)} {
		input := validTestAffiliatePayout()
		input.ExchangeRate = rate
		_, err := normalizeAffiliatePayout(input, 68)
		require.ErrorIs(t, err, ErrAffiliatePayoutInvalid)
	}
	for _, amount := range []float64{0, -1, 1e-8, math.NaN(), math.Inf(1)} {
		_, err := normalizeAffiliatePayout(validTestAffiliatePayout(), amount)
		require.ErrorIs(t, err, ErrAffiliatePayoutInvalid)
	}
	for _, mutate := range []func(*AffiliatePayout){
		func(p *AffiliatePayout) { p.Network = "tron" },
		func(p *AffiliatePayout) { p.WalletAddress = "0x" + strings.Repeat("0", 40) },
		func(p *AffiliatePayout) { p.WalletAddress = "not-an-address" },
		func(p *AffiliatePayout) { p.TxHash = "https://bscscan.com/tx/abc" },
		func(p *AffiliatePayout) { p.TxHash = "0x" + strings.Repeat("g", 64) },
		func(p *AffiliatePayout) { p.Note = strings.Repeat("字", 2001) },
	} {
		input := validTestAffiliatePayout()
		mutate(&input)
		_, err := normalizeAffiliatePayout(input, 68)
		require.ErrorIs(t, err, ErrAffiliatePayoutInvalid)
	}
}

type affiliatePayoutSettingStub struct {
	SettingRepository
	value string
}

func (s affiliatePayoutSettingStub) GetValue(context.Context, string) (string, error) {
	return s.value, nil
}

func TestAffiliatePayoutRate_ConfigAndFallback(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want float64
	}{
		{"", 6.8}, {"NaN", 6.8}, {"0", 6.8}, {"+Inf", 6.8}, {"6.95", 6.95}, {"7.123456789", 7.12345679},
	} {
		svc := &SettingService{settingRepo: affiliatePayoutSettingStub{value: tc.raw}}
		require.Equal(t, tc.want, svc.GetAffiliateUSDTExchangeRate(context.Background()))
	}
}

func TestAdminWithdrawUSDT_RequiresProofBeforeDeduction(t *testing.T) {
	repo := &affiliateWithdrawRepoStub{}
	svc := NewAffiliateService(repo, nil, nil, nil, &TicketService{})
	_, err := svc.AdminWithdrawUSDT(context.Background(), 42, 1, 68, testAffiliateWithdrawKey, validTestAffiliatePayout(), nil)
	require.ErrorIs(t, err, ErrAffiliatePayoutInvalid)
	require.Empty(t, repo.calls)
}
