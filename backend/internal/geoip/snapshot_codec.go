package geoip

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"

	"geoatlas/internal/model"
)

const (
	snapshotMagic   = "NMGS"
	snapshotVersion = uint32(1)
)

// SourceStamp — дешёвый отпечаток содержимого geo_ranges (все строки, до compact skip).
// Сверяется с агрегатами ClickHouse, без полного скана при старте.
type SourceStamp struct {
	Count    uint64
	MinStart uint32
	MaxEnd   uint32
	SumStart uint64
	SumEnd   uint64
	XorSpan  uint64
}

func (a SourceStamp) Equal(b SourceStamp) bool { return a == b }

func (s *SourceStamp) add(start, end uint32) {
	if s.Count == 0 {
		s.MinStart = start
		s.MaxEnd = end
	} else {
		if start < s.MinStart {
			s.MinStart = start
		}
		if end > s.MaxEnd {
			s.MaxEnd = end
		}
	}
	s.Count++
	s.SumStart += uint64(start)
	s.SumEnd += uint64(end)
	s.XorSpan ^= (uint64(start) << 32) | uint64(end)
}

// StampFromRanges — тот же отпечаток, что QuerySourceStamp по geo_ranges.
func StampFromRanges(ranges []model.GeoRange) SourceStamp {
	var s SourceStamp
	for _, g := range ranges {
		s.add(g.StartIP, g.EndIP)
	}
	return s
}

// EncodeSnapshot сериализует compact snapshot + stamp источника (CH).
func EncodeSnapshot(built *BuiltSnapshot, stamp SourceStamp) ([]byte, error) {
	snap := &snapshot{}
	if built != nil && built.snap != nil {
		snap = built.snap
	}
	payload := encodePayload(snap, stamp)
	sum := crc32.ChecksumIEEE(payload)
	out := make([]byte, 0, 8+len(payload)+4)
	out = append(out, snapshotMagic...)
	out = binary.LittleEndian.AppendUint32(out, snapshotVersion)
	out = append(out, payload...)
	out = binary.LittleEndian.AppendUint32(out, sum)
	return out, nil
}

// DecodeSnapshot читает EncodeSnapshot. Повреждённый файл — ошибка, не partial index.
func DecodeSnapshot(data []byte) (*BuiltSnapshot, SourceStamp, error) {
	var zero SourceStamp
	if len(data) < 12 {
		return nil, zero, errors.New("geo snapshot: truncated")
	}
	if string(data[:4]) != snapshotMagic {
		return nil, zero, errors.New("geo snapshot: bad magic")
	}
	ver := binary.LittleEndian.Uint32(data[4:8])
	if ver != snapshotVersion {
		return nil, zero, errors.New("geo snapshot: unsupported version")
	}
	sum := binary.LittleEndian.Uint32(data[len(data)-4:])
	payload := data[8 : len(data)-4]
	if crc32.ChecksumIEEE(payload) != sum {
		return nil, zero, errors.New("geo snapshot: checksum mismatch")
	}
	snap, stamp, err := decodePayload(payload)
	if err != nil {
		return nil, zero, err
	}
	return &BuiltSnapshot{snap: snap}, stamp, nil
}

func encodePayload(snap *snapshot, stamp SourceStamp) []byte {
	var b []byte
	b = binary.LittleEndian.AppendUint64(b, stamp.Count)
	b = binary.LittleEndian.AppendUint32(b, stamp.MinStart)
	b = binary.LittleEndian.AppendUint32(b, stamp.MaxEnd)
	b = binary.LittleEndian.AppendUint64(b, stamp.SumStart)
	b = binary.LittleEndian.AppendUint64(b, stamp.SumEnd)
	b = binary.LittleEndian.AppendUint64(b, stamp.XorSpan)
	b = appendDict(b, snap.countries)
	b = appendDict(b, snap.regions)
	b = appendDict(b, snap.cities)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(snap.rows)))
	for _, r := range snap.rows {
		b = binary.LittleEndian.AppendUint32(b, r.StartIP)
		b = binary.LittleEndian.AppendUint32(b, r.EndIP)
		b = binary.LittleEndian.AppendUint32(b, r.CountryID)
		b = binary.LittleEndian.AppendUint32(b, r.RegionID)
		b = binary.LittleEndian.AppendUint32(b, r.CityID)
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(r.Lat))
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(r.Lon))
	}
	return b
}

func decodePayload(p []byte) (*snapshot, SourceStamp, error) {
	var stamp SourceStamp
	r := &byteReader{p: p}
	var err error
	if stamp.Count, err = r.u64(); err != nil {
		return nil, stamp, err
	}
	if stamp.MinStart, err = r.u32(); err != nil {
		return nil, stamp, err
	}
	if stamp.MaxEnd, err = r.u32(); err != nil {
		return nil, stamp, err
	}
	if stamp.SumStart, err = r.u64(); err != nil {
		return nil, stamp, err
	}
	if stamp.SumEnd, err = r.u64(); err != nil {
		return nil, stamp, err
	}
	if stamp.XorSpan, err = r.u64(); err != nil {
		return nil, stamp, err
	}
	countries, err := r.dict()
	if err != nil {
		return nil, stamp, err
	}
	regions, err := r.dict()
	if err != nil {
		return nil, stamp, err
	}
	cities, err := r.dict()
	if err != nil {
		return nil, stamp, err
	}
	n, err := r.u32()
	if err != nil {
		return nil, stamp, err
	}
	rows := make([]rangeRow, n)
	for i := range rows {
		if rows[i].StartIP, err = r.u32(); err != nil {
			return nil, stamp, err
		}
		if rows[i].EndIP, err = r.u32(); err != nil {
			return nil, stamp, err
		}
		if rows[i].CountryID, err = r.u32(); err != nil {
			return nil, stamp, err
		}
		if rows[i].RegionID, err = r.u32(); err != nil {
			return nil, stamp, err
		}
		if rows[i].CityID, err = r.u32(); err != nil {
			return nil, stamp, err
		}
		var bits uint64
		if bits, err = r.u64(); err != nil {
			return nil, stamp, err
		}
		rows[i].Lat = math.Float64frombits(bits)
		if bits, err = r.u64(); err != nil {
			return nil, stamp, err
		}
		rows[i].Lon = math.Float64frombits(bits)
	}
	if r.off != len(r.p) {
		return nil, stamp, errors.New("geo snapshot: trailing bytes")
	}
	snap := &snapshot{
		rows:      rows,
		countries: countries,
		regions:   regions,
		cities:    cities,
	}
	snap.approxBytes = estimateSnapshotBytes(snap)
	return snap, stamp, nil
}

func estimateSnapshotBytes(snap *snapshot) uint64 {
	if snap == nil {
		return 0
	}
	n := uint64(cap(snap.rows)) * rangeRowSize
	n += uint64(cap(snap.countries))*stringHeaderSize + dictBytes(snap.countries)
	n += uint64(cap(snap.regions))*stringHeaderSize + dictBytes(snap.regions)
	n += uint64(cap(snap.cities))*stringHeaderSize + dictBytes(snap.cities)
	return n
}

func dictBytes(values []string) uint64 {
	var n uint64
	for _, s := range values {
		n += uint64(len(s))
	}
	return n
}

func appendDict(b []byte, values []string) []byte {
	b = binary.LittleEndian.AppendUint32(b, uint32(len(values)))
	for _, s := range values {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(s)))
		b = append(b, s...)
	}
	return b
}

type byteReader struct {
	p   []byte
	off int
}

func (r *byteReader) u32() (uint32, error) {
	if r.off+4 > len(r.p) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint32(r.p[r.off:])
	r.off += 4
	return v, nil
}

func (r *byteReader) u64() (uint64, error) {
	if r.off+8 > len(r.p) {
		return 0, io.ErrUnexpectedEOF
	}
	v := binary.LittleEndian.Uint64(r.p[r.off:])
	r.off += 8
	return v, nil
}

func (r *byteReader) dict() ([]string, error) {
	n, err := r.u32()
	if err != nil {
		return nil, err
	}
	out := make([]string, n)
	for i := range out {
		ln, err := r.u32()
		if err != nil {
			return nil, err
		}
		if r.off+int(ln) > len(r.p) {
			return nil, io.ErrUnexpectedEOF
		}
		out[i] = string(r.p[r.off : r.off+int(ln)])
		r.off += int(ln)
	}
	return out, nil
}
