package service

import "testing"

// 修正系数是比价排序的乘数，非法值必须收敛成「不修正」而不是 0：
// 0 会让所有分组的比价倍率变成 0 并排到最前，结论正好反了。
func TestNormalizeRateCorrection(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"零值按不修正", 0, 1.0},
		{"负数按不修正", -2.5, 1.0},
		{"充值十倍", 0.1, 0.1},
		{"一比一充值", 1.0, 1.0},
		{"大于一也允许", 2.0, 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeRateCorrection(tt.input); got != tt.want {
				t.Errorf("NormalizeRateCorrection(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUpstreamGroupComparisonCorrectedRate(t *testing.T) {
	exclusive := 0.8

	tests := []struct {
		name       string
		group      UpstreamGroup
		correction float64
		want       float64
	}{
		{
			name:       "基础倍率乘修正系数",
			group:      UpstreamGroup{RateMultiplier: 1.0},
			correction: 0.1,
			want:       0.1,
		},
		{
			name:       "专属倍率优先于基础倍率",
			group:      UpstreamGroup{RateMultiplier: 1.0, EffectiveRateMultiplier: &exclusive},
			correction: 0.5,
			want:       0.4,
		},
		{
			name:       "缺失修正系数等于不修正",
			group:      UpstreamGroup{RateMultiplier: 0.3},
			correction: 0,
			want:       0.3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := UpstreamGroupComparison{
				UpstreamGroup:          tt.group,
				ProviderRateCorrection: tt.correction,
			}
			if got := c.CorrectedRate(); got != tt.want {
				t.Errorf("CorrectedRate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// 回归用例：这正是引入修正系数要解决的场景——充值 10 倍、倍率 1.0 的上游
// 真实成本比 1:1 充值、倍率 0.2 的上游更低，只看声明倍率会得出相反结论。
func TestCorrectedRateReordersAgainstDeclaredRate(t *testing.T) {
	cheapAfterCorrection := UpstreamGroupComparison{
		UpstreamGroup:          UpstreamGroup{RateMultiplier: 1.0},
		ProviderRateCorrection: 0.1,
	}
	expensiveAfterCorrection := UpstreamGroupComparison{
		UpstreamGroup:          UpstreamGroup{RateMultiplier: 0.2},
		ProviderRateCorrection: 1.0,
	}

	if !(cheapAfterCorrection.ComparableRate() > expensiveAfterCorrection.ComparableRate()) {
		t.Fatal("前提不成立：声明倍率本应是 1.0 > 0.2")
	}
	if !(cheapAfterCorrection.CorrectedRate() < expensiveAfterCorrection.CorrectedRate()) {
		t.Errorf(
			"修正后应该反转顺序：%v 应小于 %v",
			cheapAfterCorrection.CorrectedRate(), expensiveAfterCorrection.CorrectedRate(),
		)
	}
}
