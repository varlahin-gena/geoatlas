package systemlive

import (
	"network_monitor/internal/installprofile"
	"network_monitor/internal/usecase/system"
)

// ProfileAdapter loads installation profiles from disk.
type ProfileAdapter struct{}

var _ system.ProfileLoader = ProfileAdapter{}

func (ProfileAdapter) Load(path string) (*system.CapacityProfile, error) {
	profile, err := installprofile.Load(path)
	if err != nil {
		return nil, err
	}
	return &system.CapacityProfile{
		GeneratedAt: profile.GeneratedAt, Profile: profile.Profile, ProfileLabel: profile.ProfileLabel,
		Host: system.ProfileHost{
			CPUCores: profile.Host.CPUCores, RAMMB: profile.Host.RAMMB,
			DiskGBAvail: profile.Host.DiskGBAvail, Cgroup: profile.Host.Cgroup,
		},
		Limits: system.ProfileLimits{
			ClickHouse: system.ProfileClickHouseLimits{
				ProfileServiceLimits: system.ProfileServiceLimits{
					MemoryGB: profile.Limits.ClickHouse.MemoryGB, CPUs: profile.Limits.ClickHouse.CPUs,
				},
				MaxQueryMemoryBytes: profile.Limits.ClickHouse.MaxQueryMemoryBytes,
				ExternalSpillBytes:  profile.Limits.ClickHouse.ExternalSpillBytes,
			},
			Backend: system.ProfileBackendLimits{
				ProfileServiceLimits: system.ProfileServiceLimits{
					MemoryGB: profile.Limits.Backend.MemoryGB, CPUs: profile.Limits.Backend.CPUs,
				},
				IngestWorkers: profile.Limits.Backend.IngestWorkers,
				IngestQueueSize: profile.Limits.Backend.IngestQueueSize,
				IngestBatchSize: profile.Limits.Backend.IngestBatchSize,
			},
			SyslogNG: system.ProfileSyslogLimits{
				MemoryMB: profile.Limits.SyslogNG.MemoryMB, CPUs: profile.Limits.SyslogNG.CPUs,
				FifoSize: profile.Limits.SyslogNG.FifoSize, MemBufBytes: profile.Limits.SyslogNG.MemBufBytes,
				DiskBufBytes: profile.Limits.SyslogNG.DiskBufBytes, UDPRcvbufBytes: profile.Limits.SyslogNG.UDPRcvbufBytes,
				IWSize: profile.Limits.SyslogNG.IWSize,
			},
		},
		Capacity: system.ProfileCapacity{
			ExpectedEPSMin: profile.Capacity.ExpectedEPSMin, ExpectedEPSMax: profile.Capacity.ExpectedEPSMax,
		},
	}, nil
}
