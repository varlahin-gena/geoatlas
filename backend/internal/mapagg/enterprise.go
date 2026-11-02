package mapagg

import (
	"net"
	"sort"
	"strings"

	"geoatlas/internal/model"
)

// EnterpriseHit — метки сети предприятия, покрывающей IP.
type EnterpriseHit struct {
	City    string
	Country string
	Region  string
	Network string
	Label   string
}

// EnterpriseIndex — отсортированные CIDR-диапазоны enterprise_nets для Contains/Lookup.
type EnterpriseIndex struct {
	ranges []enterpriseRange
}

type enterpriseRange struct {
	Start   uint32
	End     uint32
	City    string
	Country string
	Region  string
	Network string
	Label   string
}

// NewEnterpriseIndex строит индекс по списку сетей (пустой/nil → всегда miss).
func NewEnterpriseIndex(nets []model.EnterpriseNet) *EnterpriseIndex {
	if len(nets) == 0 {
		return &EnterpriseIndex{}
	}
	ranges := make([]enterpriseRange, 0, len(nets))
	for _, n := range nets {
		start, end := n.StartIP, n.EndIP
		if end < start {
			continue
		}
		ranges = append(ranges, enterpriseRange{
			Start:   start,
			End:     end,
			City:    strings.TrimSpace(n.City),
			Country: strings.TrimSpace(n.Country),
			Region:  strings.TrimSpace(n.Region),
			Network: n.Network,
			Label:   n.Label,
		})
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start == ranges[j].Start {
			return ranges[i].End < ranges[j].End
		}
		return ranges[i].Start < ranges[j].Start
	})
	return &EnterpriseIndex{ranges: ranges}
}

// Empty — нет сетей (все IP → country в hybrid-режиме).
func (idx *EnterpriseIndex) Empty() bool {
	return idx == nil || len(idx.ranges) == 0
}

// Lookup ищет покрытие IP; ok=false если индекс пуст или IP вне периметра.
func (idx *EnterpriseIndex) Lookup(ipStr string) (EnterpriseHit, bool) {
	if idx.Empty() {
		return EnterpriseHit{}, false
	}
	ip := ipToUint32(ipStr)
	if ip == 0 && strings.TrimSpace(ipStr) != "0.0.0.0" {
		return EnterpriseHit{}, false
	}
	// Все кандидаты с Start <= ip (вложенные CIDR; max ~200).
	i := sort.Search(len(idx.ranges), func(i int) bool {
		return idx.ranges[i].Start > ip
	})
	for j := i - 1; j >= 0; j-- {
		r := idx.ranges[j]
		if r.Start <= ip && ip <= r.End {
			return EnterpriseHit{
				City:    r.City,
				Country: r.Country,
				Region:  r.Region,
				Network: r.Network,
				Label:   r.Label,
			}, true
		}
	}
	return EnterpriseHit{}, false
}

// Contains — IP внутри хотя бы одной enterprise-сети.
func (idx *EnterpriseIndex) Contains(ipStr string) bool {
	_, ok := idx.Lookup(ipStr)
	return ok
}

func ipToUint32(ipStr string) uint32 {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return 0
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return 0
	}
	v4 := ip.To4()
	if v4 == nil {
		return 0
	}
	return uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
}
