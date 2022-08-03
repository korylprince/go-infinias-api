package api

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnsuccessfulRequest = errors.New("unsuccessful request")

type Error struct {
	ID  string `json:"id"`
	Msg string `json:"msg"`
}

func (e *Error) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s: %s", e.ID, e.Msg)
	}
	return e.Msg
}

type Errors []*Error

func (e Errors) Error() string {
	strs := make([]string, len(e))
	for idx, err := range e {
		strs[idx] = err.Error()
	}

	return strings.Join(strs, ", ")
}

func errorMatchesString(err error, s string) bool {
	var errs Errors
	if !errors.As(err, &errs) {
		return false
	}

	for _, e := range errs {
		if strings.Contains(strings.ToLower(e.Msg), s) {
			return true
		}
	}

	return false
}

func IsBadgeExistsError(err error) bool {
	return errorMatchesString(err, "badge credential could not be created because it already exists") ||
		errorMatchesString(err, "badge credential could not be updated because it already exists")
}

func IsNotFoundError(err error) bool {
	return errorMatchesString(err, "notfound")
}
