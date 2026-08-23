package geo

import (
	"context"
	"io"
	"testing"
	"time"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
)

type memEnterprise struct {
	nets map[[2]uint32]model.EnterpriseNet
}

func (m *memEnterprise) ListEnterpriseNets(context.Context) ([]model.EnterpriseNet, error) {
	out := make([]model.EnterpriseNet, 0, len(m.nets))
	for _, n := range m.nets {
		out = append(out, n)
	}
	return out, nil
}

func (m *memEnterprise) UpsertEnterpriseNet(_ context.Context, net model.EnterpriseNet) error {
	if m.nets == nil {
		m.nets = map[[2]uint32]model.EnterpriseNet{}
	}
	m.nets[[2]uint32{net.StartIP, net.EndIP}] = net
	return nil
}

func (m *memEnterprise) DeleteEnterpriseNet(_ context.Context, startIP, endIP uint32) error {
	delete(m.nets, [2]uint32{startIP, endIP})
	return nil
}

func (m *memEnterprise) CountEnterpriseNets(context.Context) (int, error) {
	return len(m.nets), nil
}

type parseCodec struct{}

func (parseCodec) ReadCSV(io.Reader) ([]model.GeoRange, error) { return nil, nil }
func (parseCodec) ReadCSVSnapshot(io.Reader) ([]model.GeoRange, *geoip.BuiltSnapshot, error) {
	return nil, nil, nil
}
func (parseCodec) WriteCSV(io.Writer, []model.GeoRange) error { return nil }
func (parseCodec) Normalize(ranges []model.GeoRange) ([]model.GeoRange, int) {
	return ranges, 0
}
func (parseCodec) CheckNonOverlapping([]model.GeoRange) error { return nil }
func (parseCodec) ParseEntry(string, string, string, string, float64, float64) (model.GeoRange, error) {
	return model.GeoRange{}, nil
}
func (parseCodec) ParseNetwork(network string) (uint32, uint32, bool) {
	return geoip.ParseNetworkField(network)
}
func (parseCodec) FormatNetwork(start, end uint32) string {
	return geoip.FormatNetwork(start, end)
}

func TestAddEnterpriseNetCIDR(t *testing.T) {
	st := &memEnterprise{}
	s := New(nil, nil, nil, nil, parseCodec{}, 0)
	s.SetEnterpriseStore(st)
	n, err := s.AddEnterpriseNet(context.Background(), AddEnterpriseNetInput{
		Network: "10.20.0.0/16",
		City:    "Москва",
		Country: "Россия",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n.StartIP == 0 || n.EndIP <= n.StartIP {
		t.Fatalf("range: %+v", n)
	}
	list, err := s.ListEnterpriseNets(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if err := s.DeleteEnterpriseNet(context.Background(), n.StartIP, n.EndIP); err != nil {
		t.Fatal(err)
	}
	list, err = s.ListEnterpriseNets(context.Background())
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete list=%v err=%v", list, err)
	}
}

func TestAddEnterpriseNetRejectsBad(t *testing.T) {
	s := New(nil, nil, nil, nil, parseCodec{}, 0)
	s.SetEnterpriseStore(&memEnterprise{})
	if _, err := s.AddEnterpriseNet(context.Background(), AddEnterpriseNetInput{Network: "not-an-ip"}); err == nil {
		t.Fatal("expected invalid")
	}
}

func TestAddEnterpriseNetCap(t *testing.T) {
	st := &memEnterprise{nets: map[[2]uint32]model.EnterpriseNet{}}
	for i := 0; i < MaxEnterpriseNets; i++ {
		u := uint32(i + 1)
		st.nets[[2]uint32{u, u}] = model.EnterpriseNet{StartIP: u, EndIP: u, CreatedAt: time.Now()}
	}
	s := New(nil, nil, nil, nil, parseCodec{}, 0)
	s.SetEnterpriseStore(st)
	if _, err := s.AddEnterpriseNet(context.Background(), AddEnterpriseNetInput{Network: "10.0.0.1"}); err == nil {
		t.Fatal("expected cap")
	}
}
