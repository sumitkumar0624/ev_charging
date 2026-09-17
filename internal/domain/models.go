package domain

import "time"

type Driver struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Vehicle   Vehicle   `json:"vehicle"`
	CreatedAt time.Time `json:"created_at"`
}

type Vehicle struct {
	ID                      string          `json:"id"`
	DriverID                string          `json:"driver_id"`
	RegistrationNumber      string          `json:"registration_number"`
	SupportedConnectorTypes []ConnectorType `json:"supported_connector_types"`
}

func (v Vehicle) Supports(t ConnectorType) bool {
	for _, supported := range v.SupportedConnectorTypes {
		if supported == t {
			return true
		}
	}
	return false
}

type Station struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Latitude   float64     `json:"latitude"`
	Longitude  float64     `json:"longitude"`
	Connectors []Connector `json:"connectors"`
	CreatedAt  time.Time   `json:"created_at"`
}

type Connector struct {
	ID        string          `json:"id"`
	StationID string          `json:"station_id"`
	Type      ConnectorType   `json:"type"`
	PowerKW   float64         `json:"power_kw"`
	Status    ConnectorStatus `json:"status"`
}

type TariffTier struct {
	UpToKWh         *float64 `json:"up_to_kwh,omitempty"`
	RatePaisePerKWh int64    `json:"rate_paise_per_kwh"`
}

type Tariff struct {
	ID                 string        `json:"id"`
	ConnectorType      ConnectorType `json:"connector_type"`
	MinimumChargePaise int64         `json:"minimum_charge_paise"`
	Tiers              []TariffTier  `json:"tiers"`
}

type PromoCode struct {
	Code            string    `json:"code"`
	DiscountPercent int       `json:"discount_percent"`
	ValidFrom       time.Time `json:"valid_from"`
	ValidUntil      time.Time `json:"valid_until"`
	MaxUses         *int      `json:"max_uses,omitempty"`
	UsedCount       int       `json:"used_count"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}

func (p PromoCode) IsValid(at time.Time) bool {
	return p.Active && !at.Before(p.ValidFrom) && !at.After(p.ValidUntil) &&
		(p.MaxUses == nil || p.UsedCount < *p.MaxUses)
}

type Session struct {
	ID                    string        `json:"id"`
	DriverID              string        `json:"driver_id"`
	VehicleID             string        `json:"vehicle_id"`
	StationID             string        `json:"station_id"`
	ConnectorID           string        `json:"connector_id"`
	PhysicalConnectorType ConnectorType `json:"physical_connector_type"`
	BilledConnectorType   ConnectorType `json:"billed_connector_type"`
	Status                SessionStatus `json:"status"`
	PromoCode             *string       `json:"promo_code,omitempty"`
	PromoDiscountPercent  int           `json:"promo_discount_percent"`
	EnergyDeliveredKWh    *float64      `json:"energy_delivered_kwh,omitempty"`
	BaseCostPaise         *int64        `json:"base_cost_paise,omitempty"`
	DiscountPaise         *int64        `json:"discount_paise,omitempty"`
	FinalCostPaise        *int64        `json:"final_cost_paise,omitempty"`
	StartedAt             time.Time     `json:"started_at"`
	EndedAt               *time.Time    `json:"ended_at,omitempty"`
}
