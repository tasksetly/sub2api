package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type provisionAccountAdminStub struct {
	AdminService
	account *Account
}

func (s *provisionAccountAdminStub) CreateAccount(_ context.Context, input *CreateAccountInput) (*Account, error) {
	s.account = &Account{ID: 1, Name: input.Name, Type: input.Type, Platform: input.Platform, Credentials: input.Credentials}
	return s.account, nil
}

func (s *provisionAccountAdminStub) GetAccount(context.Context, int64) (*Account, error) {
	return s.account, nil
}

func (s *provisionAccountAdminStub) UpdateAccount(context.Context, int64, *UpdateAccountInput) (*Account, error) {
	return s.account, nil
}

func TestUpstreamProviderProvisionAccountPlatform(t *testing.T) {
	const root = "https://relay.example.com"
	tests := []struct {
		name         string
		providerType string
		group        string
		override     string
		wantPlatform string
		wantBaseURL  string
	}{
		{"inherit upstream group", UpstreamProviderTypeSub2API, PlatformGemini, "", PlatformGemini, root},
		{"inherit another group", UpstreamProviderTypeSub2API, PlatformGrok, "", PlatformGrok, root},
		{"default for empty group platform", UpstreamProviderTypeSub2API, "", "", PlatformAnthropic, root},
		{"override upstream group", UpstreamProviderTypeSub2API, PlatformAnthropic, PlatformCommandCode, PlatformCommandCode, root},
		{"default for NewAPI", UpstreamProviderTypeNewAPI, "", "", PlatformOpenAI, root + "/v1"},
		{"override NewAPI default", UpstreamProviderTypeNewAPI, PlatformOpenAI, PlatformGemini, PlatformGemini, root},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &UpstreamProviderService{adminService: &provisionAccountAdminStub{}}
			account, err := svc.createLocalAccount(context.Background(),
				&UpstreamProvider{ID: 1, ProviderType: tt.providerType, BaseURL: root},
				&UpstreamGroup{RemoteGroupID: 7, Platform: tt.group},
				&UpstreamCreatedKey{Key: "test-key"}, "test-account", ProvisionAccountInput{Platform: tt.override},
			)
			require.NoError(t, err)
			require.Equal(t, tt.wantPlatform, account.Platform)
			require.Equal(t, tt.wantBaseURL, account.GetCredential("base_url"))
			if tt.wantPlatform == PlatformCommandCode {
				require.Equal(t, root, account.GetCNProtocolBaseURL(APIProtocolAnthropic))
				require.Equal(t, root, account.GetCNProtocolBaseURL(APIProtocolResponses))
			}
		})
	}
}
