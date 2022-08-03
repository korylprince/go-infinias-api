package infinias

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/korylprince/go-infinias-api/api"
	"github.com/korylprince/go-infinias-api/db"
)

type HTTPError struct {
	StatusCode int
	Err        error
}

func (h *HTTPError) Error() string {
	return fmt.Sprintf("%d %s: %s", h.StatusCode, http.StatusText(h.StatusCode), h.Err.Error())
}

func (h *HTTPError) Unwrap() error {
	return h.Err
}

func HTTPErrorCode(err error) int {
	h := new(HTTPError)
	if errors.As(err, &h) {
		return h.StatusCode
	}
	return http.StatusInternalServerError
}

type jsonResponse struct {
	Code        int    `json:"code"`
	Description string `json:"description"`
}

func (s *Service) HandleJSON(next func(r *http.Request) (interface{}, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		code := http.StatusOK
		resp, err := next(r)
		if err != nil {
			if s.Log != nil {
				s.Log(fmt.Sprintf("%s %s: %v", r.Method, r.URL.String(), err))
			}
			code = HTTPErrorCode(err)
			resp = &jsonResponse{Code: code, Description: err.Error()}
		}

		w.WriteHeader(code)
		if err = json.NewEncoder(w).Encode(resp); err != nil {
			if s.Log != nil {
				s.Log(fmt.Sprintf("%s %s: could not encode: %v", r.Method, r.URL.String(), err))
			}
		}
	})
}

func (s *Service) Handler() http.Handler {
	mux := mux.NewRouter()

	mux.Path("/people").Methods(http.MethodPost).Handler(s.HandleJSON(s.CreatePersonHandler))
	mux.Path("/people/{id}").Methods(http.MethodGet).Handler(s.HandleJSON(s.ReadPersonHandler))
	mux.Path("/people/{id}").Methods(http.MethodPut).Handler(s.HandleJSON(s.UpdatePersonHandler))
	mux.Path("/people/{id}").Methods(http.MethodDelete).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok := &jsonResponse{Code: http.StatusOK, Description: "200 OK"}
		err := s.DeletePersonHandler(r)
		s.HandleJSON(func(r *http.Request) (interface{}, error) {
			return ok, err
		}).ServeHTTP(w, r)
	})
	mux.Path("/people").Methods(http.MethodGet).Handler(s.HandleJSON(s.ListPersonHandler))
	mux.Path("/groups").Methods(http.MethodGet).Handler(s.HandleJSON(s.ListGroupsHandler))

	return s.WithAuth(mux)
}

func (s *Service) CreatePersonHandler(r *http.Request) (interface{}, error) {
	p := new(Person)
	if err := json.NewDecoder(r.Body).Decode(p); err != nil {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read body: %w", err)}
	}

	id, err := s.CreatePerson(p)
	if err != nil {
		code := http.StatusInternalServerError
		if api.IsBadgeExistsError(err) {
			code = http.StatusConflict
		}
		return nil, &HTTPError{StatusCode: code, Err: fmt.Errorf("could not create person: %w", err)}
	}

	p.ID = id
	p.HasImage = len(p.Image) != 0
	p.Image = nil
	p.GroupsToAdd = nil

	return p, nil
}

func (s *Service) ReadPersonHandler(r *http.Request) (interface{}, error) {
	idStr := mux.Vars(r)["id"]
	if idStr == "" {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", ErrInvalidID)}
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", err)}
	}

	p, err := s.ReadPerson(id)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrInvalidID {
			code = http.StatusBadRequest
		} else if api.IsNotFoundError(err) {
			code = http.StatusNotFound
		}
		return nil, &HTTPError{StatusCode: code, Err: fmt.Errorf("could not update person: %w", err)}
	}

	return p, nil
}

func (s *Service) UpdatePersonHandler(r *http.Request) (interface{}, error) {
	idStr := mux.Vars(r)["id"]
	if idStr == "" {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", ErrInvalidID)}
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", err)}
	}

	p := new(Person)
	if err := json.NewDecoder(r.Body).Decode(p); err != nil {
		return nil, &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read body: %w", err)}
	}

	p.ID = id

	// client can't set this
	p.HasImage = false

	if err := s.UpdatePerson(p); err != nil {
		code := http.StatusInternalServerError
		if err == ErrInvalidID {
			code = http.StatusBadRequest
		} else if api.IsNotFoundError(err) {
			code = http.StatusNotFound
		} else if api.IsBadgeExistsError(err) {
			code = http.StatusConflict
		}
		return nil, &HTTPError{StatusCode: code, Err: fmt.Errorf("could not update person: %w", err)}
	}

	p.GroupsToAdd = nil

	// if image was just updated without error, then has_image is true
	if len(p.Image) != 0 {
		p.HasImage = true
		p.Image = nil
		return p, nil
	}

	if _, err := s.DBConn.ReadPicture(p.ID); err != nil {
		if err == db.ErrNotFound {
			return p, nil
		}
		return nil, &HTTPError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("could not read picture: %w", err)}
	}

	p.HasImage = true

	return p, nil
}

func (s *Service) DeletePersonHandler(r *http.Request) error {
	idStr := mux.Vars(r)["id"]
	if idStr == "" {
		return &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", ErrInvalidID)}
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return &HTTPError{StatusCode: http.StatusBadRequest, Err: fmt.Errorf("could not read id: %w", err)}
	}

	if err := s.DeletePerson(id); err != nil {
		code := http.StatusInternalServerError
		if api.IsNotFoundError(err) {
			code = http.StatusNotFound
		}
		return &HTTPError{StatusCode: code, Err: fmt.Errorf("could not delete person: %w", err)}
	}

	return nil
}

func (s *Service) ListPersonHandler(r *http.Request) (interface{}, error) {
	people, err := s.ListPeople()
	if err != nil {
		return nil, &HTTPError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("could not list people: %w", err)}
	}

	return people, nil
}

func (s *Service) ListGroupsHandler(r *http.Request) (interface{}, error) {
	groups, err := s.ListGroups()
	if err != nil {
		return nil, &HTTPError{StatusCode: http.StatusInternalServerError, Err: fmt.Errorf("could not list groups: %w", err)}
	}

	return groups, nil
}
