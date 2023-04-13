package infinias

import (
	"errors"
	"fmt"

	"github.com/korylprince/go-infinias-api/api"
	"github.com/korylprince/go-infinias-api/db"
)

var ErrInvalidID = errors.New("invalid id")

type Person struct {
	ID          int           `json:"id"`
	FirstName   string        `json:"first_name"`
	LastName    string        `json:"last_name"`
	EmployeeID  string        `json:"employee_id"`
	Department  string        `json:"department"`
	SiteCode    int           `json:"site_code"`
	CardCode    int           `json:"card_code"`
	Image       []byte        `json:"image,omitempty"`
	HasImage    bool          `json:"has_image"`
	GroupsToAdd []int         `json:"groups_to_add,omitempty"`
	Credentials []*Credential `json:"credentials,omitempty"`
}

type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Service struct {
	APIConn *api.Conn
	DBConn  *db.Conn
	Log     func(string)
	APIKey  string
}

func (s *Service) CreatePerson(p *Person) (int, error) {
	id, err := s.APIConn.CreatePerson(&api.Person{
		FirstName:   p.FirstName,
		LastName:    p.LastName,
		EmployeeID:  p.EmployeeID,
		Department:  p.Department,
		SiteCode:    p.SiteCode,
		CardCode:    p.CardCode,
		GroupsToAdd: p.GroupsToAdd,
	})
	if err != nil {
		return 0, fmt.Errorf("could not create person: %w", err)
	}

	if p.Image == nil {
		return id, nil
	}

	if err = s.DBConn.UpdatePicture(id, p.Image); err != nil {
		return 0, fmt.Errorf("could not update picture: %w", err)
	}

	for _, cred := range p.Credentials {
		if cred.SiteCode == p.SiteCode && cred.CardCode == p.CardCode {
			continue
		}

		if _, err := s.DBConn.CreateCredential(id, (*db.Credential)(cred)); err != nil {
			return 0, fmt.Errorf("could not create credential (%d-%d): %w", cred.SiteCode, cred.CardCode, err)
		}
	}

	return id, nil
}

func (s *Service) ReadPerson(id int) (*Person, error) {
	p, err := s.APIConn.ReadPerson(id)
	if err != nil {
		return nil, fmt.Errorf("could not read person: %w", err)
	}

	buf, err := s.DBConn.ReadPicture(id)
	if err != nil {
		if err != db.ErrNotFound {
			return nil, fmt.Errorf("could not read picture: %w", err)
		}
	}

	creds, err := s.DBConn.ListCredentials(id)
	if err != nil {
		return nil, fmt.Errorf("could not read credentials: %w", err)
	}

	newcreds := make([]*Credential, len(creds))
	for idx, c := range creds {
		newcreds[idx] = (*Credential)(c)
	}

	return &Person{
		ID:          p.ID,
		FirstName:   p.FirstName,
		LastName:    p.LastName,
		EmployeeID:  p.EmployeeID,
		Department:  p.Department,
		SiteCode:    p.SiteCode,
		CardCode:    p.CardCode,
		HasImage:    len(buf) != 0,
		Image:       buf,
		Credentials: newcreds,
	}, nil
}

func (s *Service) UpdatePerson(p *Person) error {
	if p.ID == 0 {
		return ErrInvalidID
	}
	if err := s.APIConn.UpdatePerson(&api.Person{
		ID:          p.ID,
		FirstName:   p.FirstName,
		LastName:    p.LastName,
		EmployeeID:  p.EmployeeID,
		Department:  p.Department,
		SiteCode:    p.SiteCode,
		CardCode:    p.CardCode,
		GroupsToAdd: p.GroupsToAdd,
	}); err != nil {
		return fmt.Errorf("could not update person: %w", err)
	}

	if len(p.Image) == 0 {
		return nil
	}

	if err := s.DBConn.UpdatePicture(p.ID, p.Image); err != nil {
		return fmt.Errorf("could not update picture: %w", err)
	}

	for _, cred := range p.Credentials {
		if cred.SiteCode == p.SiteCode && cred.CardCode == p.CardCode {
			continue
		}

		if _, err := s.DBConn.CreateCredential(p.ID, (*db.Credential)(cred)); err != nil {
			return fmt.Errorf("could not create credential (%d-%d): %w", cred.SiteCode, cred.CardCode, err)
		}
	}

	return nil
}

func (s *Service) DeletePerson(id int) error {
	if err := s.APIConn.DeletePerson(id); err != nil {
		return fmt.Errorf("could not delete person: %w", err)
	}
	return nil
}

func (s *Service) ListPeople() ([]*Person, error) {
	apiPeople, err := s.APIConn.ListPeople()
	if err != nil {
		return nil, fmt.Errorf("could not list people: %w", err)
	}

	ids, err := s.DBConn.HasPictureIDs()
	if err != nil {
		return nil, fmt.Errorf("could not list picture ids: %w", err)
	}

	idSet := make(map[int]struct{})
	for _, i := range ids {
		idSet[i] = struct{}{}
	}

	depts, err := s.DBConn.ListDepartments()
	if err != nil {
		return nil, fmt.Errorf("could not list departments: %w", err)
	}

	credMap, err := s.DBConn.ListAllCredentials()
	if err != nil {
		return nil, fmt.Errorf("could not list credentials: %w", err)
	}

	people := make([]*Person, len(apiPeople))
	for idx, p := range apiPeople {
		_, ok := idSet[p.ID]

		var newcreds []*Credential
		if creds := credMap[p.ID]; len(creds) > 0 {
			newcreds = make([]*Credential, len(creds))
			for idx, c := range creds {
				newcreds[idx] = (*Credential)(c)
			}
		}

		people[idx] = &Person{
			ID:          p.ID,
			FirstName:   p.FirstName,
			LastName:    p.LastName,
			EmployeeID:  p.EmployeeID,
			Department:  depts[p.ID],
			SiteCode:    p.SiteCode,
			CardCode:    p.CardCode,
			HasImage:    ok,
			Credentials: newcreds,
		}
	}

	return people, nil
}

func (s *Service) ListGroups() ([]*Group, error) {
	apiGroups, err := s.APIConn.ListGroups()
	if err != nil {
		return nil, fmt.Errorf("could not list groups: %w", err)
	}

	groups := make([]*Group, len(apiGroups))
	for idx, g := range apiGroups {
		groups[idx] = &Group{
			ID:   g.ID,
			Name: g.Name,
		}
	}

	return groups, nil
}

type Credential struct {
	ID       int  `json:"id,omitempty"`
	Active   bool `json:"active"`
	SiteCode int  `json:"site_code"`
	CardCode int  `json:"card_code"`
}

func (s *Service) CreateCredential(id int, cred *Credential) (int, error) {
	credID, err := s.DBConn.CreateCredential(id, (*db.Credential)(cred))
	if err != nil {
		return 0, err
	}

	return credID, nil
}

func (s *Service) DeleteCredential(id, credID int) error {
	if err := s.DBConn.DeleteCredential(id, credID); err != nil {
		return err
	}

	return nil
}

func (s *Service) ListCredentials(id int) ([]*Credential, error) {
	creds, err := s.DBConn.ListCredentials(id)
	if err != nil {
		return nil, err
	}

	newcreds := make([]*Credential, len(creds))
	for idx, c := range creds {
		newcreds[idx] = (*Credential)(c)
	}

	return newcreds, nil
}
