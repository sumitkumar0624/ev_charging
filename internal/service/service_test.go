package service

import (
	"context"
	"testing"
	"time"

	"github.com/example/evcharging/internal/billing"
	"github.com/example/evcharging/internal/domain"
	"github.com/example/evcharging/internal/models"
	"github.com/example/evcharging/internal/repository"
)

func TestACRequestUsesDCConnectorButBillsACTariff(t *testing.T) {
	repo := &fakeRepository{
		driver: domain.Driver{ID: "driver-1", Vehicle: domain.Vehicle{
			ID: "vehicle-1", SupportedConnectorTypes: []domain.ConnectorType{domain.ConnectorAC, domain.ConnectorDC},
		}},
	}
	svc := New(repo, billing.TieredCalculator{})
	svc.now = func() time.Time { return time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC) }

	started, err := svc.StartSession(context.Background(), models.StartSessionInput{
		DriverID: "driver-1", RequestedType: domain.ConnectorAC,
		Latitude: 12.9716, Longitude: 77.5946, RadiusKM: 5,
	})
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}
	if started.PhysicalConnectorType != domain.ConnectorDC {
		t.Fatalf("physical type = %s, want DC", started.PhysicalConnectorType)
	}
	if started.BilledConnectorType != domain.ConnectorAC {
		t.Fatalf("billed type = %s, want AC", started.BilledConnectorType)
	}
	if !repo.claim.AllowDCFallback {
		t.Fatal("expected DC fallback to be enabled for a dual-compatible vehicle")
	}

	completed, err := svc.EndSession(context.Background(), started.ID, 10)
	if err != nil {
		t.Fatalf("EndSession() error = %v", err)
	}
	if repo.requestedTariff != domain.ConnectorAC {
		t.Fatalf("tariff requested for %s, want AC", repo.requestedTariff)
	}
	if completed.FinalCostPaise == nil || *completed.FinalCostPaise != 12000 {
		t.Fatalf("final price = %v, want 12000 paise", completed.FinalCostPaise)
	}
}

type fakeRepository struct {
	driver          domain.Driver
	session         domain.Session
	claim           repository.ClaimRequest
	requestedTariff domain.ConnectorType
}

func (f *fakeRepository) CreateDriver(_ context.Context, d domain.Driver) (domain.Driver, error) {
	return d, nil
}
func (f *fakeRepository) GetDriver(_ context.Context, _ string) (domain.Driver, error) {
	return f.driver, nil
}
func (f *fakeRepository) CreateStation(_ context.Context, s domain.Station) (domain.Station, error) {
	return s, nil
}
func (f *fakeRepository) CreatePromo(_ context.Context, p domain.PromoCode) (domain.PromoCode, error) {
	return p, nil
}
func (f *fakeRepository) DeletePromo(context.Context, string) error { return nil }
func (f *fakeRepository) ClaimConnector(_ context.Context, request repository.ClaimRequest) (domain.Session, error) {
	f.claim = request
	f.session = domain.Session{
		ID: request.SessionID, DriverID: request.DriverID, VehicleID: request.VehicleID,
		StationID: "station-1", ConnectorID: "dc-connector-1",
		PhysicalConnectorType: domain.ConnectorDC, BilledConnectorType: request.RequestedType,
		Status: domain.SessionActive, StartedAt: request.StartedAt,
	}
	return f.session, nil
}
func (f *fakeRepository) GetSession(context.Context, string) (domain.Session, error) {
	return f.session, nil
}
func (f *fakeRepository) GetTariff(_ context.Context, connectorType domain.ConnectorType) (domain.Tariff, error) {
	f.requestedTariff = connectorType
	return domain.Tariff{
		ConnectorType: domain.ConnectorAC, MinimumChargePaise: 10000,
		Tiers: []domain.TariffTier{
			{UpToKWh: pointer(10), RatePaisePerKWh: 1200},
			{UpToKWh: nil, RatePaisePerKWh: 900},
		},
	}, nil
}
func (f *fakeRepository) CompleteSession(_ context.Context, _ string, energy float64, price billing.Price, ended time.Time) (domain.Session, error) {
	f.session.Status = domain.SessionCompleted
	f.session.EnergyDeliveredKWh = &energy
	f.session.BaseCostPaise, f.session.DiscountPaise, f.session.FinalCostPaise =
		&price.BasePaise, &price.DiscountPaise, &price.FinalPaise
	f.session.EndedAt = &ended
	return f.session, nil
}
func (f *fakeRepository) SetConnectorStatus(_ context.Context, _ string, status domain.ConnectorStatus) (domain.Connector, error) {
	return domain.Connector{Status: status}, nil
}
func (f *fakeRepository) ListDriverSessions(context.Context, string) ([]domain.Session, error) {
	return nil, nil
}
func (f *fakeRepository) ListStationSessions(context.Context, string) ([]domain.Session, error) {
	return nil, nil
}

func pointer(value float64) *float64 { return &value }
