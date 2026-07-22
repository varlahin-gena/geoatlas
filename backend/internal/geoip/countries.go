package geoip

import "network_monitor/internal/model"

// CountryCenters / CountryCenter — совместимость; SoT в model.
var CountryCenters = model.CountryCenters

func CountryCenter(name string) (float64, float64, bool) {
	return model.CountryCenter(name)
}
