package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/example/evcharging/internal/billing"
	"github.com/example/evcharging/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func Connect(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return NewPostgres(pool), nil
}

func (p *Postgres) Close() {
	if p != nil && p.pool != nil {
		p.pool.Close()
	}
}

func (p *Postgres) CreateDriver(ctx context.Context, driver domain.Driver) (domain.Driver, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Driver{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `INSERT INTO drivers (id, name, email, created_at) VALUES ($1,$2,$3,$4)`,
		driver.ID, driver.Name, driver.Email, driver.CreatedAt)
	if err == nil {
		types := make([]string, len(driver.Vehicle.SupportedConnectorTypes))
		for i, connectorType := range driver.Vehicle.SupportedConnectorTypes {
			types[i] = string(connectorType)
		}
		_, err = tx.Exec(ctx, `INSERT INTO vehicles
			(id, driver_id, email, registration_number, supported_connector_types)
			VALUES ($1,$2,$3,$4,$5)`, driver.Vehicle.ID, driver.ID, driver.Email,
			driver.Vehicle.RegistrationNumber, types)
	}
	if err != nil {
		return domain.Driver{}, mapError(err)
	}
	return driver, tx.Commit(ctx)
}

func (p *Postgres) GetDriver(ctx context.Context, id string) (domain.Driver, error) {
	var d domain.Driver
	var supported []string
	err := p.pool.QueryRow(ctx, `SELECT d.id,d.name,d.email,d.created_at,
		v.id,v.driver_id,v.registration_number,v.supported_connector_types
		FROM drivers d JOIN vehicles v ON v.driver_id=d.id WHERE d.id=$1`, id).
		Scan(&d.ID, &d.Name, &d.Email, &d.CreatedAt, &d.Vehicle.ID,
			&d.Vehicle.DriverID, &d.Vehicle.RegistrationNumber, &supported)
	for _, connectorType := range supported {
		d.Vehicle.SupportedConnectorTypes = append(d.Vehicle.SupportedConnectorTypes, domain.ConnectorType(connectorType))
	}
	return d, mapError(err)
}

func (p *Postgres) CreateStation(ctx context.Context, station domain.Station) (domain.Station, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Station{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO stations (id,name,latitude,longitude,created_at)
		VALUES ($1,$2,$3,$4,$5)`, station.ID, station.Name, station.Latitude, station.Longitude, station.CreatedAt)
	for _, connector := range station.Connectors {
		if err != nil {
			break
		}
		_, err = tx.Exec(ctx, `INSERT INTO connectors (id,station_id,type,power_kw,status)
			VALUES ($1,$2,$3,$4,$5)`, connector.ID, station.ID, connector.Type, connector.PowerKW, connector.Status)
	}
	if err != nil {
		return domain.Station{}, mapError(err)
	}
	return station, tx.Commit(ctx)
}

func (p *Postgres) CreatePromo(ctx context.Context, promo domain.PromoCode) (domain.PromoCode, error) {
	_, err := p.pool.Exec(ctx, `INSERT INTO promo_codes
		(code,discount_percent,valid_from,valid_until,max_uses,used_count,active,created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, promo.Code, promo.DiscountPercent,
		promo.ValidFrom, promo.ValidUntil, promo.MaxUses, promo.UsedCount, promo.Active, promo.CreatedAt)
	return promo, mapError(err)
}

func (p *Postgres) DeletePromo(ctx context.Context, code string) error {
	tag, err := p.pool.Exec(ctx, `UPDATE promo_codes SET active=false WHERE code=$1 AND active=true`, strings.ToUpper(code))
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapError(err)
}

func (p *Postgres) ClaimConnector(ctx context.Context, request ClaimRequest) (domain.Session, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Session{}, err
	}
	defer tx.Rollback(ctx)

	discount := 0
	var promoCode *string
	if request.PromoCode != nil {
		code := strings.ToUpper(strings.TrimSpace(*request.PromoCode))
		var promo domain.PromoCode
		err = tx.QueryRow(ctx, `SELECT code,discount_percent,valid_from,valid_until,max_uses,used_count,active,created_at
			FROM promo_codes WHERE code=$1 FOR UPDATE`, code).
			Scan(&promo.Code, &promo.DiscountPercent, &promo.ValidFrom, &promo.ValidUntil,
				&promo.MaxUses, &promo.UsedCount, &promo.Active, &promo.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !promo.IsValid(request.StartedAt)) {
			return domain.Session{}, domain.ErrInvalidPromo
		}
		if err != nil {
			return domain.Session{}, err
		}
		discount, promoCode = promo.DiscountPercent, &code
	}

	allowedTypes := []string{string(request.RequestedType)}
	if request.RequestedType == domain.ConnectorAC && request.AllowDCFallback {
		allowedTypes = append(allowedTypes, string(domain.ConnectorDC))
	}
	var connector domain.Connector
	err = tx.QueryRow(ctx, `SELECT c.id,c.station_id,c.type,c.power_kw,c.status
		FROM connectors c JOIN stations s ON s.id=c.station_id
		WHERE c.status='AVAILABLE' AND c.type = ANY($1)
		  AND 6371 * 2 * ASIN(SQRT(
		    POWER(SIN(RADIANS(s.latitude-$2)/2),2) +
		    COS(RADIANS($2))*COS(RADIANS(s.latitude))*
		    POWER(SIN(RADIANS(s.longitude-$3)/2),2)
		  )) <= $4
		ORDER BY
		  CASE WHEN c.type=$5 THEN 0 ELSE 1 END,
		  6371 * 2 * ASIN(SQRT(
		    POWER(SIN(RADIANS(s.latitude-$2)/2),2) +
		    COS(RADIANS($2))*COS(RADIANS(s.latitude))*
		    POWER(SIN(RADIANS(s.longitude-$3)/2),2)
		  )),
		  c.power_kw DESC, c.id
		FOR UPDATE OF c SKIP LOCKED LIMIT 1`, allowedTypes, request.Latitude,
		request.Longitude, request.RadiusKM, request.RequestedType).
		Scan(&connector.ID, &connector.StationID, &connector.Type, &connector.PowerKW, &connector.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrNoConnectorAvailable
	}
	if err != nil {
		return domain.Session{}, err
	}

	tag, err := tx.Exec(ctx, `UPDATE connectors SET status='IN_USE' WHERE id=$1 AND status='AVAILABLE'`, connector.ID)
	if err != nil || tag.RowsAffected() != 1 {
		if err != nil {
			return domain.Session{}, err
		}
		return domain.Session{}, domain.ErrNoConnectorAvailable
	}
	if promoCode != nil {
		_, err = tx.Exec(ctx, `UPDATE promo_codes SET used_count=used_count+1 WHERE code=$1`, *promoCode)
		if err != nil {
			return domain.Session{}, err
		}
	}

	session := domain.Session{
		ID: request.SessionID, DriverID: request.DriverID, VehicleID: request.VehicleID,
		StationID: connector.StationID, ConnectorID: connector.ID,
		PhysicalConnectorType: connector.Type, BilledConnectorType: request.RequestedType,
		Status: domain.SessionActive, PromoCode: promoCode, PromoDiscountPercent: discount,
		StartedAt: request.StartedAt,
	}
	_, err = tx.Exec(ctx, `INSERT INTO charging_sessions
		(id,driver_id,vehicle_id,station_id,connector_id,physical_connector_type,
		 billed_connector_type,status,promo_code,promo_discount_percent,started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, session.ID, session.DriverID,
		session.VehicleID, session.StationID, session.ConnectorID, session.PhysicalConnectorType,
		session.BilledConnectorType, session.Status, session.PromoCode,
		session.PromoDiscountPercent, session.StartedAt)
	if err != nil {
		return domain.Session{}, mapError(err)
	}
	return session, tx.Commit(ctx)
}

func (p *Postgres) GetTariff(ctx context.Context, connectorType domain.ConnectorType) (domain.Tariff, error) {
	var tariff domain.Tariff
	err := p.pool.QueryRow(ctx, `SELECT id,connector_type,minimum_charge_paise
		FROM tariffs WHERE connector_type=$1`, connectorType).
		Scan(&tariff.ID, &tariff.ConnectorType, &tariff.MinimumChargePaise)
	if err != nil {
		return domain.Tariff{}, mapError(err)
	}
	rows, err := p.pool.Query(ctx, `SELECT up_to_kwh,rate_paise_per_kwh
		FROM tariff_tiers WHERE tariff_id=$1 ORDER BY position`, tariff.ID)
	if err != nil {
		return domain.Tariff{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var tier domain.TariffTier
		if err = rows.Scan(&tier.UpToKWh, &tier.RatePaisePerKWh); err != nil {
			return domain.Tariff{}, err
		}
		tariff.Tiers = append(tariff.Tiers, tier)
	}
	return tariff, rows.Err()
}

func (p *Postgres) GetSession(ctx context.Context, id string) (domain.Session, error) {
	return scanSession(p.pool.QueryRow(ctx, `SELECT `+sessionColumns+` FROM charging_sessions WHERE id=$1`, id))
}

func (p *Postgres) CompleteSession(ctx context.Context, id string, energy float64, price billing.Price, endedAt time.Time) (domain.Session, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	defer tx.Rollback(ctx)
	session, err := getSession(ctx, tx, id, true)
	if err != nil {
		return domain.Session{}, err
	}
	if session.Status != domain.SessionActive {
		return domain.Session{}, domain.ErrSessionNotActive
	}
	_, err = tx.Exec(ctx, `UPDATE charging_sessions SET status='COMPLETED',
		energy_delivered_kwh=$2,base_cost_paise=$3,discount_paise=$4,final_cost_paise=$5,ended_at=$6
		WHERE id=$1`, id, energy, price.BasePaise, price.DiscountPaise, price.FinalPaise, endedAt)
	if err == nil {
		_, err = tx.Exec(ctx, `UPDATE connectors SET status='AVAILABLE' WHERE id=$1 AND status='IN_USE'`, session.ConnectorID)
	}
	if err != nil {
		return domain.Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Session{}, err
	}
	session.Status, session.EnergyDeliveredKWh = domain.SessionCompleted, &energy
	session.BaseCostPaise, session.DiscountPaise, session.FinalCostPaise = &price.BasePaise, &price.DiscountPaise, &price.FinalPaise
	session.EndedAt = &endedAt
	return session, nil
}

func (p *Postgres) SetConnectorStatus(ctx context.Context, id string, status domain.ConnectorStatus) (domain.Connector, error) {
	var connector domain.Connector
	err := p.pool.QueryRow(ctx, `UPDATE connectors SET status=$2
		WHERE id=$1 AND status<>'IN_USE'
		RETURNING id,station_id,type,power_kw,status`, id, status).
		Scan(&connector.ID, &connector.StationID, &connector.Type, &connector.PowerKW, &connector.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		checkErr := p.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM connectors WHERE id=$1)`, id).Scan(&exists)
		if checkErr != nil {
			return domain.Connector{}, checkErr
		}
		if exists {
			return domain.Connector{}, domain.ErrConnectorInUse
		}
	}
	return connector, mapError(err)
}

func (p *Postgres) ListDriverSessions(ctx context.Context, email string) ([]domain.Session, error) {
	return p.listSessions(ctx, `SELECT `+sessionColumns+` FROM charging_sessions
		WHERE driver_id IN (SELECT id FROM drivers WHERE email=$1)
		ORDER BY started_at DESC`, strings.ToLower(strings.TrimSpace(email)))
}

func (p *Postgres) ListStationSessions(ctx context.Context, id string) ([]domain.Session, error) {
	return p.listSessions(ctx, `SELECT `+sessionColumns+` FROM charging_sessions WHERE station_id=$1 ORDER BY started_at DESC`, id)
}

const sessionColumns = `id,driver_id,vehicle_id,station_id,connector_id,physical_connector_type,
	billed_connector_type,status,promo_code,promo_discount_percent,energy_delivered_kwh,
	base_cost_paise,discount_paise,final_cost_paise,started_at,ended_at`

type scanner interface{ Scan(...any) error }

func scanSession(row scanner) (domain.Session, error) {
	var s domain.Session
	err := row.Scan(&s.ID, &s.DriverID, &s.VehicleID, &s.StationID, &s.ConnectorID,
		&s.PhysicalConnectorType, &s.BilledConnectorType, &s.Status, &s.PromoCode,
		&s.PromoDiscountPercent, &s.EnergyDeliveredKWh, &s.BaseCostPaise,
		&s.DiscountPaise, &s.FinalCostPaise, &s.StartedAt, &s.EndedAt)
	return s, mapError(err)
}

func getSession(ctx context.Context, tx pgx.Tx, id string, lock bool) (domain.Session, error) {
	query := `SELECT ` + sessionColumns + ` FROM charging_sessions WHERE id=$1`
	if lock {
		query += ` FOR UPDATE`
	}
	return scanSession(tx.QueryRow(ctx, query, id))
}

func (p *Postgres) listSessions(ctx context.Context, query, id string) ([]domain.Session, error) {
	rows, err := p.pool.Query(ctx, query, id)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	sessions := make([]domain.Session, 0)
	for rows.Next() {
		session, scanErr := scanSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, session)
	}
	return sessions, mapError(rows.Err())
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%w: duplicate value", domain.ErrConflict)
		case "22P02":
			return fmt.Errorf("%w: identifier must be a UUID", domain.ErrInvalidInput)
		}
	}
	return err
}
