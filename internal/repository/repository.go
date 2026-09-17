package repository

import (
	"context"
	"time"

	"github.com/example/evcharging/internal/billing"
	"github.com/example/evcharging/internal/domain"
)

type ClaimRequest struct {
	SessionID       string
	DriverID        string
	VehicleID       string
	RequestedType   domain.ConnectorType
	AllowDCFallback bool
	Latitude        float64
	Longitude       float64
	RadiusKM        float64
	PromoCode       *string
	StartedAt       time.Time
}

type Repository interface {
	CreateDriver(context.Context, domain.Driver) (domain.Driver, error)
	GetDriver(context.Context, string) (domain.Driver, error)
	CreateStation(context.Context, domain.Station) (domain.Station, error)
	CreatePromo(context.Context, domain.PromoCode) (domain.PromoCode, error)
	DeletePromo(context.Context, string) error
	ClaimConnector(context.Context, ClaimRequest) (domain.Session, error)
	GetSession(context.Context, string) (domain.Session, error)
	GetTariff(context.Context, domain.ConnectorType) (domain.Tariff, error)
	CompleteSession(context.Context, string, float64, billing.Price, time.Time) (domain.Session, error)
	SetConnectorStatus(context.Context, string, domain.ConnectorStatus) (domain.Connector, error)
	ListDriverSessions(context.Context, string) ([]domain.Session, error)
	ListStationSessions(context.Context, string) ([]domain.Session, error)
}
