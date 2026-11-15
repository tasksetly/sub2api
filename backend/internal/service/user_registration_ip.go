package service

import (
	"context"
	"net/netip"
)

type registrationIPContextKey struct{}

// WithRegistrationIP is only installed on public authentication routes. Admin
// account creation must never record the administrator's address as a signup IP.
func WithRegistrationIP(ctx context.Context, clientIP string) context.Context {
	addr, err := netip.ParseAddr(clientIP)
	if err != nil || addr.Zone() != "" {
		return ctx
	}
	return context.WithValue(ctx, registrationIPContextKey{}, addr.Unmap().String())
}

func RegistrationIPFromContext(ctx context.Context) string {
	clientIP, _ := ctx.Value(registrationIPContextKey{}).(string)
	return clientIP
}

type userRegistrationIPReader interface {
	GetRegistrationIP(context.Context, int64) (string, error)
}

type DeleteUserResult struct {
	Message                   string `json:"message"`
	RegistrationIP            string `json:"registration_ip,omitempty"`
	RegistrationIPBlacklisted bool   `json:"registration_ip_blacklisted"`
}
