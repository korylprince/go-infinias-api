package infinias

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

var ErrInvalidAuthorization = errors.New("invalid authorization")

func (s *Service) WithAuth(next http.Handler) http.Handler {
	if s.APIKey == "" {
		return next
	}
	key := []byte(s.APIKey)
	keylen := int32(len(key))
	errHandler := s.HandleJSON(func(r *http.Request) (interface{}, error) {
		return nil, &HTTPError{StatusCode: http.StatusUnauthorized, Err: ErrInvalidAuthorization}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := strings.Split(r.Header.Get("Authorization"), " ")
		if len(header) != 2 || header[0] != "Bearer" {
			errHandler.ServeHTTP(w, r)
			return
		}

		if subtle.ConstantTimeEq(keylen, int32(len([]byte(header[1])))) != 1 {
			errHandler.ServeHTTP(w, r)
			return
		}

		if subtle.ConstantTimeCompare(key, []byte(header[1])) != 1 {
			errHandler.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
