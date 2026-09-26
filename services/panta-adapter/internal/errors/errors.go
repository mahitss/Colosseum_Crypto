package errors

import (
	"net/http"
	"time"
)

type Kind string

const (
	KindConfiguration  Kind = "configuration"
	KindAuthentication Kind = "authentication"
	KindAuthorization  Kind = "authorization"
	KindRateLimit      Kind = "rate_limit"
	KindValidation     Kind = "validation"
	KindNotFound       Kind = "not_found"
	KindUpstream       Kind = "upstream"
	KindTimeout        Kind = "timeout"
	KindNetwork        Kind = "network"
	KindMalformed      Kind = "malformed_response"
)

// Error is a sanitized Panta error. It never includes response bodies or credentials.
type Error struct {
	Kind         Kind
	StatusCode   int
	RequestID    string
	UpstreamCode string
	RetryAfter   time.Duration
}

func (e *Error) Error() string {
	if e == nil {
		return "Panta request failed"
	}
	switch e.Kind {
	case KindConfiguration:
		return "Panta client configuration is invalid"
	case KindAuthentication:
		return "Panta authentication failed"
	case KindAuthorization:
		return "Panta authorization failed"
	case KindRateLimit:
		return "Panta rate limit exceeded"
	case KindValidation:
		return "Panta rejected the request"
	case KindNotFound:
		return "Panta resource was not found"
	case KindTimeout:
		return "Panta request timed out"
	case KindMalformed:
		return "Panta returned a malformed response"
	case KindNetwork:
		return "Panta is unreachable"
	default:
		return "Panta request failed"
	}
}

func (e *Error) Retryable() bool {
	return e != nil && (e.Kind == KindNetwork || e.Kind == KindTimeout || e.Kind == KindRateLimit || e.StatusCode >= 500)
}

func ForStatus(status int) Kind {
	switch status {
	case http.StatusUnauthorized:
		return KindAuthentication
	case http.StatusForbidden:
		return KindAuthorization
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return KindValidation
	case http.StatusNotFound:
		return KindNotFound
	case http.StatusTooManyRequests:
		return KindRateLimit
	default:
		if status >= http.StatusInternalServerError {
			return KindUpstream
		}
		return KindUpstream
	}
}

func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && other.Kind != "" && e.Kind == other.Kind
}

func New(kind Kind) *Error {
	if kind == "" {
		return nil
	}
	return &Error{Kind: kind}
}

func WithStatus(status int, requestID, upstreamCode string, retryAfter time.Duration) *Error {
	return &Error{
		Kind:         ForStatus(status),
		StatusCode:   status,
		RequestID:    requestID,
		UpstreamCode: upstreamCode,
		RetryAfter:   retryAfter,
	}
}
