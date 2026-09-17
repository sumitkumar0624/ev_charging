package domain

import "errors"

type ConnectorType string

const (
	ConnectorAC ConnectorType = "AC"
	ConnectorDC ConnectorType = "DC"
)

func (t ConnectorType) Valid() bool { return t == ConnectorAC || t == ConnectorDC }

type ConnectorStatus string

const (
	ConnectorAvailable    ConnectorStatus = "AVAILABLE"
	ConnectorInUse        ConnectorStatus = "IN_USE"
	ConnectorOutOfService ConnectorStatus = "OUT_OF_SERVICE"
)

func (s ConnectorStatus) ValidForManualUpdate() bool {
	return s == ConnectorAvailable || s == ConnectorOutOfService
}

type SessionStatus string

const (
	SessionActive    SessionStatus = "ACTIVE"
	SessionCompleted SessionStatus = "COMPLETED"
)

var (
	ErrNotFound             = errors.New("resource not found")
	ErrConflict             = errors.New("resource conflict")
	ErrNoConnectorAvailable = errors.New("no compatible connector available within radius")
	ErrInvalidPromo         = errors.New("promo code is invalid or inactive")
	ErrConnectorInUse       = errors.New("connector is currently in use")
	ErrSessionNotActive     = errors.New("session is not active")
	ErrUnsupportedConnector = errors.New("vehicle does not support requested connector type")
	ErrInvalidInput         = errors.New("invalid input")
)
