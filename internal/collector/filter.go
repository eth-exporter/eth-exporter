package collector

import "regexp"

// DeviceFilter selects which network interfaces to collect.
type DeviceFilter struct {
	Include *regexp.Regexp
	Exclude *regexp.Regexp
}

// Match reports whether name should be collected.
func (f DeviceFilter) Match(name string) bool {
	if f.Include != nil {
		return f.Include.MatchString(name)
	}
	if f.Exclude != nil {
		return !f.Exclude.MatchString(name)
	}
	return true
}
