package errors

import (
	"net/http"
	"testing"
)

func TestStatusKinds(t *testing.T) {
	cases := map[int]Kind{
		http.StatusUnauthorized:        KindAuthentication,
		http.StatusForbidden:           KindAuthorization,
		http.StatusBadRequest:          KindValidation,
		http.StatusNotFound:            KindNotFound,
		http.StatusTooManyRequests:     KindRateLimit,
		http.StatusInternalServerError: KindUpstream,
	}
	for status, want := range cases {
		if got := ForStatus(status); got != want {
			t.Errorf("ForStatus(%d) = %q, want %q", status, got, want)
		}
	}
}
