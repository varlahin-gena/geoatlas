package geo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"network_monitor/internal/apperr"
	"network_monitor/internal/model"
)

func (s *Service) ListEnterpriseNets(ctx context.Context) ([]model.EnterpriseNet, error) {
	if s == nil || s.enterprise == nil {
		return nil, fmt.Errorf("enterprise nets store unavailable")
	}
	return s.enterprise.ListEnterpriseNets(ctx)
}

type AddEnterpriseNetInput struct {
	Network string
	Label   string
	Country string
	Region  string
	City    string
}

func (s *Service) AddEnterpriseNet(ctx context.Context, in AddEnterpriseNetInput) (model.EnterpriseNet, error) {
	if s == nil || s.enterprise == nil {
		return model.EnterpriseNet{}, fmt.Errorf("enterprise nets store unavailable")
	}
	if s.codec == nil {
		return model.EnterpriseNet{}, fmt.Errorf("geo codec unavailable")
	}
	start, end, ok := s.codec.ParseNetwork(in.Network)
	if !ok || start > end {
		return model.EnterpriseNet{}, apperr.InvalidInput("invalid IPv4 network (CIDR, range or address)")
	}
	n, err := s.enterprise.CountEnterpriseNets(ctx)
	if err != nil {
		return model.EnterpriseNet{}, err
	}
	existing, err := s.enterprise.ListEnterpriseNets(ctx)
	if err != nil {
		return model.EnterpriseNet{}, err
	}
	replacing := false
	for _, e := range existing {
		if e.StartIP == start && e.EndIP == end {
			replacing = true
			break
		}
	}
	if !replacing && n >= MaxEnterpriseNets {
		return model.EnterpriseNet{}, apperr.InvalidInput("too many enterprise nets (max 200)")
	}
	net := model.EnterpriseNet{
		StartIP:   start,
		EndIP:     end,
		Network:   s.codec.FormatNetwork(start, end),
		Label:     strings.TrimSpace(in.Label),
		Country:   strings.TrimSpace(in.Country),
		Region:    strings.TrimSpace(in.Region),
		City:      strings.TrimSpace(in.City),
		CreatedAt: time.Now().UTC(),
	}
	if net.Label == "" {
		net.Label = strings.TrimSpace(strings.Join([]string{net.City, net.Region, net.Country}, ", "))
	}
	if err := s.enterprise.UpsertEnterpriseNet(ctx, net); err != nil {
		return model.EnterpriseNet{}, err
	}
	return net, nil
}

func (s *Service) DeleteEnterpriseNet(ctx context.Context, startIP, endIP uint32) error {
	if s == nil || s.enterprise == nil {
		return fmt.Errorf("enterprise nets store unavailable")
	}
	if startIP > endIP {
		return apperr.InvalidInput("invalid range")
	}
	return s.enterprise.DeleteEnterpriseNet(ctx, startIP, endIP)
}
