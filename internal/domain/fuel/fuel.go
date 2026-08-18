package fuel

import (
	"math"
	"time"

	"sanitation-operations/internal/apperror"
)

type Type string

const (
	Diesel   Type = "diesel"
	Gasoline Type = "gasoline"
	Electric Type = "electric"
)

type Unit string

const (
	Liter        Unit = "liter"
	KilowattHour Unit = "kwh"
)

type Record struct {
	ID          string    `json:"id"`
	VehicleID   string    `json:"vehicle_id"`
	FuelType    Type      `json:"fuel_type"`
	Quantity    float64   `json:"quantity"`
	Unit        Unit      `json:"unit"`
	CostCents   int64     `json:"cost_cents"`
	OdometerKm  int       `json:"odometer_km"`
	StationCode string    `json:"station_code"`
	RecordedAt  time.Time `json:"recorded_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func New(id, vehicleID string, fuelType Type, quantity float64, cost int64, odometer int, station string, recorded, now time.Time) (Record, error) {
	unit := Liter
	if fuelType == Electric {
		unit = KilowattHour
	}
	if id == "" || vehicleID == "" || station == "" || quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) || cost < 0 || odometer < 0 || recorded.After(now) {
		return Record{}, apperror.Validation(apperror.ErrValidation)
	}
	if fuelType != Diesel && fuelType != Gasoline && fuelType != Electric {
		return Record{}, apperror.Validation(apperror.ErrValidation)
	}
	return Record{ID: id, VehicleID: vehicleID, FuelType: fuelType, Quantity: quantity, Unit: unit, CostCents: cost, OdometerKm: odometer, StationCode: station, RecordedAt: recorded.UTC(), CreatedAt: now.UTC()}, nil
}

func (r Record) UnitPriceCents() float64 {
	if r.Quantity == 0 {
		return 0
	}
	return float64(r.CostCents) / r.Quantity
}
func Efficiency(previous, current Record) (float64, bool) {
	if previous.VehicleID == "" || current.VehicleID != previous.VehicleID || current.OdometerKm <= previous.OdometerKm || current.Quantity <= 0 {
		return 0, false
	}
	return float64(current.OdometerKm-previous.OdometerKm) / current.Quantity, true
}
