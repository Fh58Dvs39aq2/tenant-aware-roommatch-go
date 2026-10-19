package roommatch

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrTenantUnknown   = errors.New("tenant is not onboarded")
	ErrTenantInactive  = errors.New("tenant is inactive")
	ErrAccountInactive = errors.New("account is inactive")
)

type Ticket struct {
	TenantID  string `json:"tenant_id"`
	AccountID string `json:"account_id"`
	Region    string `json:"region"`
	PartySize int    `json:"party_size"`
}

type Match struct {
	ID       string   `json:"id"`
	TenantID string   `json:"tenant_id"`
	Region   string   `json:"region"`
	Accounts []string `json:"accounts"`
	Capacity int      `json:"capacity"`
}

type Matchmaker struct {
	mu       sync.Mutex
	tenants  map[string]bool
	accounts map[string]map[string]bool
	waiting  []Ticket
	nextID   int
}

func NewMatchmaker() *Matchmaker {
	return &Matchmaker{tenants: make(map[string]bool), accounts: make(map[string]map[string]bool)}
}

func (m *Matchmaker) SetTenant(tenantID string, active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenants[tenantID] = active
	if !active {
		kept := m.waiting[:0]
		for _, ticket := range m.waiting {
			if ticket.TenantID != tenantID {
				kept = append(kept, ticket)
			}
		}
		m.waiting = kept
	}
}

func (m *Matchmaker) SetAccount(tenantID, accountID string, active bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return ErrTenantUnknown
	}
	if m.accounts[tenantID] == nil {
		m.accounts[tenantID] = make(map[string]bool)
	}
	m.accounts[tenantID][accountID] = active
	return nil
}

func (m *Matchmaker) Enqueue(ticket Ticket) (*Match, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	active, known := m.tenants[ticket.TenantID]
	if !known {
		return nil, ErrTenantUnknown
	}
	if !active {
		return nil, ErrTenantInactive
	}
	if !m.accounts[ticket.TenantID][ticket.AccountID] {
		return nil, ErrAccountInactive
	}
	if ticket.PartySize < 1 || ticket.PartySize > 2 || ticket.Region == "" {
		return nil, errors.New("region and party_size between 1 and 2 are required")
	}
	for i, candidate := range m.waiting {
		if candidate.TenantID == ticket.TenantID && candidate.Region == ticket.Region && candidate.PartySize+ticket.PartySize <= 2 {
			m.waiting = append(m.waiting[:i], m.waiting[i+1:]...)
			m.nextID++
			return &Match{
				ID: fmt.Sprintf("match-%d", m.nextID), TenantID: ticket.TenantID, Region: ticket.Region,
				Accounts: []string{candidate.AccountID, ticket.AccountID}, Capacity: candidate.PartySize + ticket.PartySize,
			}, nil
		}
	}
	m.waiting = append(m.waiting, ticket)
	return nil, nil
}
