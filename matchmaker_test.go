package roommatch

import (
	"errors"
	"testing"
)

func TestMatchmakerTenantAndAccountDecisions(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Matchmaker)
		tickets   []Ticket
		wantMatch bool
		wantErr   error
	}{
		{
			name: "matches active accounts in one tenant and region",
			setup: func(m *Matchmaker) {
				m.SetTenant("acme", true)
				_ = m.SetAccount("acme", "ana", true)
				_ = m.SetAccount("acme", "bo", true)
			},
			tickets:   []Ticket{{TenantID: "acme", AccountID: "ana", Region: "eu", PartySize: 1}, {TenantID: "acme", AccountID: "bo", Region: "eu", PartySize: 1}},
			wantMatch: true,
		},
		{
			name: "keeps different tenants apart",
			setup: func(m *Matchmaker) {
				m.SetTenant("acme", true)
				m.SetTenant("beta", true)
				_ = m.SetAccount("acme", "ana", true)
				_ = m.SetAccount("beta", "bo", true)
			},
			tickets: []Ticket{{TenantID: "acme", AccountID: "ana", Region: "eu", PartySize: 1}, {TenantID: "beta", AccountID: "bo", Region: "eu", PartySize: 1}},
		},
		{
			name:    "rejects a suspended account",
			setup:   func(m *Matchmaker) { m.SetTenant("acme", true); _ = m.SetAccount("acme", "ana", false) },
			tickets: []Ticket{{TenantID: "acme", AccountID: "ana", Region: "eu", PartySize: 1}},
			wantErr: ErrAccountInactive,
		},
		{
			name: "removes queued tickets when tenant is suspended",
			setup: func(m *Matchmaker) {
				m.SetTenant("acme", true)
				_ = m.SetAccount("acme", "ana", true)
				_ = m.SetAccount("acme", "bo", true)
				_, _ = m.Enqueue(Ticket{TenantID: "acme", AccountID: "ana", Region: "eu", PartySize: 1})
				m.SetTenant("acme", false)
				m.SetTenant("acme", true)
			},
			tickets: []Ticket{{TenantID: "acme", AccountID: "bo", Region: "eu", PartySize: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatchmaker()
			tt.setup(matcher)
			var got *Match
			var err error
			for _, ticket := range tt.tickets {
				got, err = matcher.Enqueue(ticket)
				if err != nil {
					break
				}
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if (got != nil) != tt.wantMatch {
				t.Fatalf("match present = %v, want %v", got != nil, tt.wantMatch)
			}
		})
	}
}
