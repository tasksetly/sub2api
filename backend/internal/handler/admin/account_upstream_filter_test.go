package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestParseAccountUpstreamFilter(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{name: "empty means no filter", raw: "", want: 0},
		{name: "whitespace only means no filter", raw: "   ", want: 0},
		{name: "any keyword", raw: "any", want: service.AccountListUpstreamAny},
		{name: "specific provider", raw: "7", want: 7},
		{name: "trims surrounding space", raw: " 7 ", want: 7},
		// 0/负数/非数字都必须报错，不能退化成「不筛选」——那会让用户以为
		// 筛过了，实际看到的是全部账号。
		{name: "zero is rejected", raw: "0", wantErr: true},
		{name: "negative is rejected", raw: "-1", wantErr: true},
		{name: "non-numeric is rejected", raw: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAccountUpstreamFilter(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
