package systemlive

import (
	"network_monitor/internal/installprofile"
	"network_monitor/internal/usecase/system"
)

// ProfileAdapter loads installation profiles from disk.
type ProfileAdapter struct{}

var _ system.ProfileLoader = ProfileAdapter{}

func (ProfileAdapter) Load(path string) (*installprofile.Profile, error) {
	return installprofile.Load(path)
}
