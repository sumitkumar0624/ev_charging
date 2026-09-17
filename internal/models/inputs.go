package models

import (
	"time"

	"github.com/example/evcharging/internal/domain"
)

type RegisterDriverInput struct {
	Name                    string                 `json:"name"`
	Email                   string                 `json:"email"`
	RegistrationNumber      string                 `json:"registration_number"`
	SupportedConnectorTypes []domain.ConnectorType `json:"supported_connector_types"`
}

type ConnectorInput struct {
	Type    domain.ConnectorType `json:"type"`
	PowerKW float64              `json:"power_kw"`
}

type RegisterStationInput struct {
	Name       string           `json:"name"`
	Latitude   float64          `json:"latitude"`
	Longitude  float64          `json:"longitude"`
	Connectors []ConnectorInput `json:"connectors"`
}

type StartSessionInput struct {
	DriverID      string               `json:"driver_id"`
	RequestedType domain.ConnectorType `json:"requested_type"`
	Latitude      float64              `json:"latitude"`
	Longitude     float64              `json:"longitude"`
	RadiusKM      float64              `json:"radius_km"`
	PromoCode     *string              `json:"promo_code"`
}

type EndSessionInput struct {
	EnergyDeliveredKWh float64 `json:"energy_delivered_kwh"`
}

type SetConnectorStatusInput struct {
	Status domain.ConnectorStatus `json:"status"`
}

type CreatePromoInput struct {
	Code            string    `json:"code"`
	DiscountPercent int       `json:"discount_percent"`
	ValidFrom       time.Time `json:"valid_from"`
	ValidUntil      time.Time `json:"valid_until"`
	MaxUses         *int      `json:"max_uses"`
}
