package service

import (
	"context"
	"fmt"
	"math"
	"net/mail"
	"strings"
	"time"

	"github.com/example/evcharging/internal/billing"
	"github.com/example/evcharging/internal/domain"
	"github.com/example/evcharging/internal/models"
	"github.com/example/evcharging/internal/repository"
	"github.com/google/uuid"
)

type Clock func() time.Time

type Service struct {
	repo       repository.Repository
	calculator billing.Calculator
	now        Clock
}

func New(repo repository.Repository, calculator billing.Calculator) *Service {
	return &Service{repo: repo, calculator: calculator, now: time.Now}
}

func (s *Service) RegisterDriver(ctx context.Context, input models.RegisterDriverInput) (domain.Driver, error) {
	input.Name, input.Email = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Email))
	input.RegistrationNumber = strings.ToUpper(strings.TrimSpace(input.RegistrationNumber))
	if input.Name == "" || input.RegistrationNumber == "" || len(input.SupportedConnectorTypes) == 0 {
		return domain.Driver{}, fmt.Errorf("%w: name, registration number and connector types are required", domain.ErrInvalidInput)
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return domain.Driver{}, fmt.Errorf("%w: invalid email", domain.ErrInvalidInput)
	}
	seen := make(map[domain.ConnectorType]bool)
	for _, connectorType := range input.SupportedConnectorTypes {
		if !connectorType.Valid() {
			return domain.Driver{}, fmt.Errorf("%w: unsupported connector type %q", domain.ErrInvalidInput, connectorType)
		}
		seen[connectorType] = true
	}
	types := make([]domain.ConnectorType, 0, len(seen))
	for _, connectorType := range []domain.ConnectorType{domain.ConnectorAC, domain.ConnectorDC} {
		if seen[connectorType] {
			types = append(types, connectorType)
		}
	}
	now := s.now().UTC()
	driverID := uuid.NewString()
	driver := domain.Driver{
		ID: driverID, Name: input.Name, Email: input.Email, CreatedAt: now,
		Vehicle: domain.Vehicle{
			ID: uuid.NewString(), DriverID: driverID, RegistrationNumber: input.RegistrationNumber,
			SupportedConnectorTypes: types,
		},
	}
	return s.repo.CreateDriver(ctx, driver)
}

func (s *Service) RegisterStation(ctx context.Context, input models.RegisterStationInput) (domain.Station, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || !validCoordinates(input.Latitude, input.Longitude) || len(input.Connectors) == 0 {
		return domain.Station{}, fmt.Errorf("%w: valid name, coordinates and at least one connector are required", domain.ErrInvalidInput)
	}
	stationID := uuid.NewString()
	station := domain.Station{
		ID: stationID, Name: input.Name, Latitude: input.Latitude, Longitude: input.Longitude,
		CreatedAt: s.now().UTC(),
	}
	for _, item := range input.Connectors {
		if !item.Type.Valid() || item.PowerKW <= 0 || math.IsNaN(item.PowerKW) || math.IsInf(item.PowerKW, 0) {
			return domain.Station{}, fmt.Errorf("%w: connector type and positive power are required", domain.ErrInvalidInput)
		}
		station.Connectors = append(station.Connectors, domain.Connector{
			ID: uuid.NewString(), StationID: stationID, Type: item.Type,
			PowerKW: item.PowerKW, Status: domain.ConnectorAvailable,
		})
	}
	return s.repo.CreateStation(ctx, station)
}

func (s *Service) StartSession(ctx context.Context, input models.StartSessionInput) (domain.Session, error) {
	if input.DriverID == "" || !input.RequestedType.Valid() || !validCoordinates(input.Latitude, input.Longitude) ||
		input.RadiusKM <= 0 || math.IsNaN(input.RadiusKM) || math.IsInf(input.RadiusKM, 0) {
		return domain.Session{}, fmt.Errorf("%w: driver, connector type, coordinates and positive radius are required", domain.ErrInvalidInput)
	}
	driver, err := s.repo.GetDriver(ctx, input.DriverID)
	if err != nil {
		return domain.Session{}, err
	}
	if !driver.Vehicle.Supports(input.RequestedType) {
		return domain.Session{}, domain.ErrUnsupportedConnector
	}
	return s.repo.ClaimConnector(ctx, repository.ClaimRequest{
		SessionID: uuid.NewString(), DriverID: driver.ID, VehicleID: driver.Vehicle.ID,
		RequestedType:   input.RequestedType,
		AllowDCFallback: input.RequestedType == domain.ConnectorAC && driver.Vehicle.Supports(domain.ConnectorDC),
		Latitude:        input.Latitude, Longitude: input.Longitude, RadiusKM: input.RadiusKM,
		PromoCode: input.PromoCode, StartedAt: s.now().UTC(),
	})
}

func (s *Service) EndSession(ctx context.Context, id string, energyKWh float64) (domain.Session, error) {
	if id == "" || energyKWh < 0 || math.IsNaN(energyKWh) || math.IsInf(energyKWh, 0) {
		return domain.Session{}, fmt.Errorf("%w: session id and finite non-negative energy are required", domain.ErrInvalidInput)
	}
	session, err := s.repo.GetSession(ctx, id)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Status != domain.SessionActive {
		return domain.Session{}, domain.ErrSessionNotActive
	}
	tariff, err := s.repo.GetTariff(ctx, session.BilledConnectorType)
	if err != nil {
		return domain.Session{}, err
	}
	price, err := s.calculator.Calculate(tariff, energyKWh, session.PromoDiscountPercent)
	if err != nil {
		return domain.Session{}, err
	}
	return s.repo.CompleteSession(ctx, id, energyKWh, price, s.now().UTC())
}

func (s *Service) CreatePromo(ctx context.Context, input models.CreatePromoInput) (domain.PromoCode, error) {
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	if input.Code == "" || input.DiscountPercent < 1 || input.DiscountPercent > 100 ||
		input.ValidUntil.Before(input.ValidFrom) || (input.MaxUses != nil && *input.MaxUses <= 0) {
		return domain.PromoCode{}, fmt.Errorf("%w: invalid promo configuration", domain.ErrInvalidInput)
	}
	promo := domain.PromoCode{
		Code: input.Code, DiscountPercent: input.DiscountPercent,
		ValidFrom: input.ValidFrom.UTC(), ValidUntil: input.ValidUntil.UTC(),
		MaxUses: input.MaxUses, Active: true, CreatedAt: s.now().UTC(),
	}
	return s.repo.CreatePromo(ctx, promo)
}

func (s *Service) DeletePromo(ctx context.Context, code string) error {
	if strings.TrimSpace(code) == "" {
		return fmt.Errorf("%w: promo code is required", domain.ErrInvalidInput)
	}
	return s.repo.DeletePromo(ctx, code)
}

func (s *Service) SetConnectorStatus(ctx context.Context, id string, status domain.ConnectorStatus) (domain.Connector, error) {
	if id == "" || !status.ValidForManualUpdate() {
		return domain.Connector{}, fmt.Errorf("%w: status must be AVAILABLE or OUT_OF_SERVICE", domain.ErrInvalidInput)
	}
	return s.repo.SetConnectorStatus(ctx, id, status)
}

func (s *Service) DriverSessions(ctx context.Context, id string) ([]domain.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: driver id is required", domain.ErrInvalidInput)
	}
	return s.repo.ListDriverSessions(ctx, id)
}

func (s *Service) StationSessions(ctx context.Context, id string) ([]domain.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: station id is required", domain.ErrInvalidInput)
	}
	return s.repo.ListStationSessions(ctx, id)
}

func validCoordinates(latitude, longitude float64) bool {
	return !math.IsNaN(latitude) && !math.IsNaN(longitude) &&
		!math.IsInf(latitude, 0) && !math.IsInf(longitude, 0) &&
		latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}
