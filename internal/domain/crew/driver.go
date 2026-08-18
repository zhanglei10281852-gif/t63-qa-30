package crew

import (
	"sort"
	"time"

	"sanitation-operations/internal/apperror"
)

type Status string

const (
	Active    Status = "active"
	Suspended Status = "suspended"
	Inactive  Status = "inactive"
)

type Certification struct {
	ID          string    `json:"id"`
	DriverID    string    `json:"driver_id"`
	Code        string    `json:"code"`
	VehicleType string    `json:"vehicle_type"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Driver struct {
	ID               string          `json:"id"`
	EmployeeNo       string          `json:"employee_no"`
	Name             string          `json:"name"`
	Status           Status          `json:"status"`
	LicenseClass     string          `json:"license_class"`
	LicenseExpiresAt time.Time       `json:"license_expires_at"`
	Version          int             `json:"version"`
	Certifications   []Certification `json:"certifications"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func New(id, employeeNo, name, licenseClass string, expires, now time.Time) (Driver, error) {
	if id == "" || employeeNo == "" || name == "" || licenseClass == "" || !expires.After(now) {
		return Driver{}, apperror.Validation(apperror.ErrValidation)
	}
	return Driver{ID: id, EmployeeNo: employeeNo, Name: name, Status: Active, LicenseClass: licenseClass, LicenseExpiresAt: expires.UTC(), Version: 1, Certifications: []Certification{}, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (d Driver) AddCertification(value Certification, now time.Time) (Driver, error) {
	if d.Status == Inactive || value.Code == "" || value.VehicleType == "" || !value.ExpiresAt.After(now) {
		return Driver{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	items := append([]Certification(nil), d.Certifications...)
	replaced := false
	for index := range items {
		if items[index].Code == value.Code {
			items[index] = value
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, value)
	}
	sort.Slice(items, func(a, b int) bool { return items[a].Code < items[b].Code })
	d.Certifications = items
	d.Version++
	d.UpdatedAt = now.UTC()
	return d, nil
}

func (d Driver) CanOperate(vehicleType string, at time.Time) error {
	if d.Status != Active || !at.Before(d.LicenseExpiresAt) {
		return apperror.Conflict(apperror.ErrUnavailable)
	}
	for _, value := range d.Certifications {
		if value.VehicleType == vehicleType && at.Before(value.ExpiresAt) {
			return nil
		}
	}
	return apperror.Conflict(apperror.ErrUnavailable)
}

func (d Driver) Suspend(now time.Time) (Driver, error) {
	if d.Status != Active {
		return Driver{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	d.Status = Suspended
	d.Version++
	d.UpdatedAt = now.UTC()
	return d, nil
}
func (d Driver) Reactivate(now time.Time) (Driver, error) {
	if d.Status != Suspended || !now.Before(d.LicenseExpiresAt) {
		return Driver{}, apperror.Conflict(apperror.ErrInvalidState)
	}
	d.Status = Active
	d.Version++
	d.UpdatedAt = now.UTC()
	return d, nil
}
func (d Driver) Clone() Driver {
	d.Certifications = append([]Certification(nil), d.Certifications...)
	return d
}
