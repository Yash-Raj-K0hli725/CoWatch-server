package errz

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type Errz struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

type BadRequest struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewBadRequest(msg string) *BadRequest { return &BadRequest{msg, http.StatusBadRequest} }
func (e *BadRequest) Error() string        { return e.Message }

type Unauthorized struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewUnauthorized(msg string) *Unauthorized { return &Unauthorized{msg, http.StatusUnauthorized} }
func (e *Unauthorized) Error() string          { return e.Message }

type NotFound struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewNotFound(msg string) *NotFound { return &NotFound{msg, http.StatusNotFound} }
func (e *NotFound) Error() string      { return e.Message }

type Forbiddenn struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewForbidden(msg string) *Forbiddenn { return &Forbiddenn{msg, http.StatusForbidden} }
func (e *Forbiddenn) Error() string       { return e.Message }

type AlreadyExists struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewAlreadyExists(msg string) *AlreadyExists { return &AlreadyExists{msg, http.StatusConflict} }
func (e *AlreadyExists) Error() string           { return e.Message }

type NotAllowed struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func NewNotAllowed(msg string) *NotAllowed { return &NotAllowed{msg, http.StatusMethodNotAllowed} }
func (e NotAllowed) Error() string         { return e.Message }

func ErrzHandler(err error) error {
	var duplicate *AlreadyExists
	var notFound *NotFound
	var unauthorized *Unauthorized
	var badRequest *BadRequest
	var forbidden *Forbiddenn
	var notAllowed *NotAllowed
	switch {
	case errors.As(err, &badRequest):
		return echo.NewHTTPError(http.StatusBadRequest, err)

	case errors.As(err, &forbidden):
		return echo.NewHTTPError(http.StatusForbidden, err)

	case errors.As(err, &duplicate):
		return echo.NewHTTPError(http.StatusConflict, err)

	case errors.As(err, &notFound):
		return echo.NewHTTPError(http.StatusNotFound, err)

	case errors.As(err, &unauthorized):
		return echo.NewHTTPError(http.StatusUnauthorized, err)

	case errors.As(err, &notAllowed):
		return echo.NewHTTPError(http.StatusMethodNotAllowed, err)

	default:
		log.Errorf("something went wrong :: %+v", err)
		return err
	}
}

func FormatError(err error) Errz {
	var duplicate *AlreadyExists
	var notFound *NotFound
	var unauthorized *Unauthorized
	var badRequest *BadRequest
	var forbidden *Forbiddenn
	var notAllowed *NotAllowed

	switch {
	case errors.As(err, &forbidden):
		return Errz{
			Code: http.StatusForbidden, Message: err.Error(),
		}
	case errors.As(err, &duplicate):
		return Errz{Code: http.StatusConflict, Message: err.Error()}

	case errors.As(err, &notFound):
		return Errz{Code: http.StatusNotFound, Message: err.Error()}

	case errors.As(err, &unauthorized):
		return Errz{Code: http.StatusUnauthorized, Message: err.Error()}

	case errors.As(err, &badRequest):
		return Errz{Code: http.StatusBadRequest, Message: err.Error()}

	case errors.As(err, &notAllowed):
		return Errz{Code: http.StatusMethodNotAllowed, Message: err.Error()}

	default:
		if httpError, ok := errors.AsType[*echo.HTTPError](err); ok {
			return Errz{
				Code:    httpError.Code,
				Message: fmt.Sprint(httpError.Message),
			}
		}
		log.Errorf("something went wrong :: %+v", err)
		return Errz{Code: http.StatusInternalServerError, Message: err.Error()}
	}
}
