// Package pricing provides Lagos electricity tariff rate lookup.
// Rates are based on the Nigerian Electricity Regulatory Commission (NERC) tariff tiers.
package pricing

import "time"

// watOffset is West Africa Time = UTC+1.
const watOffset = 1 * 60 * 60

// GetCurrentRateNGN returns the grid electricity rate in NGN per kWh
// for the given UTC time, converted to West Africa Time (WAT = UTC+1).
//
// Tariff tiers:
//
//	Off-Peak  23:00–05:59 WAT → ₦185.00/kWh
//	Peak      18:00–22:59 WAT → ₦320.00/kWh
//	Shoulder  06:00–17:59 WAT → ₦225.00/kWh
func GetCurrentRateNGN(t time.Time) float64 {
	wat := time.FixedZone("WAT", watOffset)
	local := t.In(wat)
	hour := local.Hour()

	switch {
	case hour >= 23 || hour < 6:
		return 185.0 // off-peak — cheapest window, ideal for charging
	case hour >= 18 && hour < 23:
		return 320.0 // peak — most expensive, avoid charging
	default:
		return 225.0 // shoulder — mid-range
	}
}
