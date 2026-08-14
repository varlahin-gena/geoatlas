package repstore

import (
	"context"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/reputation"
	usecasereputation "network_monitor/internal/usecase/reputation"
)

// ReputationRepository реализует reputation.RangeStore.
type ReputationRepository struct {
	apiCH   clickhouse.Conn
	writeCH clickhouse.Conn
}

func NewReputationRepository(apiCH, writeCH clickhouse.Conn) *ReputationRepository {
	return &ReputationRepository{apiCH: apiCH, writeCH: writeCH}
}

var _ usecasereputation.RangeStore = (*ReputationRepository)(nil)

func (r *ReputationRepository) apiConn() clickhouse.Conn {
	if r.apiCH != nil {
		return r.apiCH
	}
	return r.writeCH
}

func (r *ReputationRepository) Load(ctx context.Context) ([]model.ReputationRange, error) {
	return LoadReputationRanges(ctx, r.writeCH)
}

func (r *ReputationRepository) ReplaceAll(ctx context.Context, ranges []model.ReputationRange) (int, error) {
	return ReplaceReputationRanges(ctx, r.writeCH, ranges)
}

func (r *ReputationRepository) ReplaceList(ctx context.Context, listName string, ranges []model.ReputationRange) (int, error) {
	all, err := LoadReputationRanges(ctx, r.writeCH)
	if err != nil {
		return 0, err
	}
	kept := make([]model.ReputationRange, 0, len(all)+len(ranges))
	for _, x := range all {
		if x.ListName != listName {
			kept = append(kept, x)
		}
	}
	kept = append(kept, ranges...)
	kept = reputation.NormalizeRanges(kept)
	if _, err := ReplaceReputationRanges(ctx, r.writeCH, kept); err != nil {
		return 0, err
	}
	// Возвращаем число диапазонов именно этого списка (после normalize),
	// а не размер всей таблицы — иначе toast refresh показывает одно число на все фиды.
	n := 0
	for _, x := range kept {
		if x.ListName == listName {
			n++
		}
	}
	return n, nil
}

func (r *ReputationRepository) DeleteList(ctx context.Context, listName string) error {
	all, err := LoadReputationRanges(ctx, r.writeCH)
	if err != nil {
		return err
	}
	kept := make([]model.ReputationRange, 0, len(all))
	for _, x := range all {
		if x.ListName != listName {
			kept = append(kept, x)
		}
	}
	_, err = ReplaceReputationRanges(ctx, r.writeCH, kept)
	return err
}

func (r *ReputationRepository) ListMeta(ctx context.Context) ([]model.ReputationListMeta, error) {
	return ListReputationMeta(ctx, r.apiConn())
}

// ReloadableReputationIndex — *reputation.Index + Reload из ClickHouse.
type ReloadableReputationIndex struct {
	*reputation.Index
	ch clickhouse.Conn
}

func NewReloadableReputationIndex(ch clickhouse.Conn) *ReloadableReputationIndex {
	return &ReloadableReputationIndex{Index: reputation.New(), ch: ch}
}

// Lookup перекрывает promoted *reputation.Index.Lookup, чтобы typed-nil
// *ReloadableReputationIndex в ReputationLookuper не паниковал.
func (i *ReloadableReputationIndex) Lookup(ipStr string) []model.ReputationHit {
	if i == nil || i.Index == nil {
		return nil
	}
	return i.Index.Lookup(ipStr)
}

func (i *ReloadableReputationIndex) Reload(ctx context.Context) error {
	if i == nil || i.Index == nil {
		return nil
	}
	ranges, err := LoadReputationRanges(ctx, i.ch)
	if err != nil {
		return err
	}
	clean := reputation.NormalizeRanges(ranges)
	i.ReplaceAll(clean)
	slog.Info("reputation index loaded", "ranges", len(clean))
	return nil
}
