package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	formKeyUsername      = "username"
	formKeyPassword      = "password"
	formKeyID            = "Id"
	formKeyFirstName     = "personalInfo.FirstName"
	formKeyLastName      = "personalInfo.LastName"
	formKeyEmployeeID    = "personalInfo.employeeId"
	formKeyDepartment    = "personalInfo.department"
	formKeySiteCode      = "badgeInfo.SiteCode"
	formKeyCardIssueCode = "badgeInfo.CardIssueCode"
	formKeyAddGroups     = "groupInfo.AddGroups"
)

var cardRegexp = regexp.MustCompile(`^(\d+)-(\d+)$`)

type Person struct {
	ID          int
	FirstName   string
	LastName    string
	EmployeeID  string
	Department  string
	SiteCode    int
	CardCode    int
	GroupsToAdd []int
}

type Group struct {
	ID   int
	Name string
}

type Conn struct {
	urlPrefix *url.URL
	username  string
	password  string
}

func (c *Conn) url() *url.URL {
	u := *(c.urlPrefix)
	return &u
}

func NewConn(urlPrefix, username, password string) (*Conn, error) {
	u, err := url.Parse(urlPrefix)
	if err != nil {
		return nil, fmt.Errorf("could not parse url prefix: %w", err)
	}
	return &Conn{urlPrefix: u, username: username, password: password}, nil
}

type Response struct {
	Success  bool        `json:"success"`
	ID       int         `json:"RecordId"`
	Data     interface{} `json:"data,omitempty"`
	Errors   `json:"errors"`
	ErrorMsg string `json:"errorMsg"`
}

func (r *Response) Error() error {
	if len(r.Errors) > 0 {
		return r.Errors
	}
	if r.ErrorMsg != "" {
		return Errors{&Error{Msg: r.ErrorMsg}}
	}
	if !r.Success {
		return ErrUnsuccessfulRequest
	}
	return nil
}

func (c *Conn) CreatePerson(p *Person) (id int, err error) {
	u := c.url()
	u.Path += "/infinias/ia/people"

	form := make(url.Values)
	form.Set(formKeyUsername, c.username)
	form.Set(formKeyPassword, c.password)
	if p.FirstName != "" {
		form.Set(formKeyFirstName, p.FirstName)
	}
	if p.LastName != "" {
		form.Set(formKeyLastName, p.LastName)
	}
	if p.EmployeeID != "" {
		form.Set(formKeyEmployeeID, p.EmployeeID)
	}
	if p.Department != "" {
		form.Set(formKeyDepartment, p.Department)
	}
	if p.SiteCode != 0 {
		form.Set(formKeySiteCode, strconv.Itoa(p.SiteCode))
	}
	if p.CardCode != 0 {
		form.Set(formKeyCardIssueCode, strconv.Itoa(p.CardCode))
	}
	if len(p.GroupsToAdd) > 0 {
		groups := make([]string, len(p.GroupsToAdd))
		for idx, g := range p.GroupsToAdd {
			groups[idx] = strconv.Itoa(g)
		}
		form.Set(formKeyAddGroups, strings.Join(groups, ","))
	}

	r, err := http.PostForm(u.String(), form)
	if err != nil {
		return 0, fmt.Errorf("could not POST person: %w", err)
	}
	defer r.Body.Close()

	resp := new(Response)
	if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
		return 0, fmt.Errorf("could not decode response body: %w", err)
	}

	if err = resp.Error(); err != nil {
		return 0, err
	}

	p.ID = resp.ID

	return resp.ID, nil
}

func (c *Conn) ReadPerson(id int) (*Person, error) {
	type data struct {
		ID           int `json:"Id"`
		PersonalInfo struct {
			FirstName  string `json:"FirstName"`
			LastName   string `json:"LastName"`
			EmployeeID string `json:"EmployeeId"`
			Department string `json:"Department"`
		} `json:"PersonalInfo"`
		BadgeInfo struct {
			SiteCode string `json:"SiteCode"`
			CardCode string `json:"CardIssueCode"`
		} `json:"BadgeInfo"`
	}

	u := c.url()
	u.Path += "/infinias/ia/people/details"
	q := u.Query()
	q.Set(formKeyUsername, c.username)
	q.Set(formKeyPassword, c.password)
	q.Set(formKeyID, strconv.Itoa(id))
	u.RawQuery = q.Encode()

	r, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("could not GET person: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		resp := new(Response)
		if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
			return nil, fmt.Errorf("could not decode response body: %w", err)
		}
		if err = resp.Error(); err != nil {
			return nil, err
		}
		if r.StatusCode == http.StatusNotFound {
			return nil, Errors{&Error{Msg: "NotFound"}}
		}
		return nil, Errors{&Error{Msg: r.Status}}
	}

	resp := new(data)
	if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
		return nil, fmt.Errorf("could not decode response body: %w", err)
	}

	sc, err := strconv.Atoi(resp.BadgeInfo.SiteCode)
	if err != nil {
		return nil, fmt.Errorf("could not parse site code: %w", err)
	}
	cc, err := strconv.Atoi(resp.BadgeInfo.CardCode)
	if err != nil {
		return nil, fmt.Errorf("could not parse card code: %w", err)
	}

	return &Person{
		ID:         resp.ID,
		FirstName:  resp.PersonalInfo.FirstName,
		LastName:   resp.PersonalInfo.LastName,
		EmployeeID: resp.PersonalInfo.EmployeeID,
		Department: resp.PersonalInfo.Department,
		SiteCode:   sc,
		CardCode:   cc,
	}, nil
}

func (c *Conn) UpdatePerson(p *Person) error {
	u := c.url()
	u.Path += "/infinias/ia/people"

	form := make(url.Values)
	form.Set(formKeyUsername, c.username)
	form.Set(formKeyPassword, c.password)
	form.Set(formKeyID, strconv.Itoa(p.ID))
	if p.FirstName != "" {
		form.Set(formKeyFirstName, p.FirstName)
	}
	if p.LastName != "" {
		form.Set(formKeyLastName, p.LastName)
	}
	if p.EmployeeID != "" {
		form.Set(formKeyEmployeeID, p.EmployeeID)
	}
	if p.Department != "" {
		form.Set(formKeyDepartment, p.Department)
	}
	if p.SiteCode != 0 {
		form.Set(formKeySiteCode, strconv.Itoa(p.SiteCode))
	}
	if p.CardCode != 0 {
		form.Set(formKeyCardIssueCode, strconv.Itoa(p.CardCode))
	}
	if len(p.GroupsToAdd) > 0 {
		groups := make([]string, len(p.GroupsToAdd))
		for idx, g := range p.GroupsToAdd {
			groups[idx] = strconv.Itoa(g)
		}
		form.Set(formKeyAddGroups, strings.Join(groups, ","))
	}

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBufferString(form.Encode()))
	if err != nil {
		return fmt.Errorf("could not create PUT request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not PUT person: %w", err)
	}
	defer r.Body.Close()

	resp := new(Response)
	if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
		return fmt.Errorf("could not decode response body: %w", err)
	}

	if err = resp.Error(); err != nil {
		return err
	}

	return nil
}

func (c *Conn) DeletePerson(id int) error {
	u := c.url()
	u.Path += "/infinias/ia/people"
	q := u.Query()
	q.Set(formKeyUsername, c.username)
	q.Set(formKeyPassword, c.password)
	q.Set(formKeyID, strconv.Itoa(id))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return fmt.Errorf("could not create DELETE request: %w", err)
	}

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not DELETE person: %w", err)
	}
	defer r.Body.Close()

	resp := new(Response)
	if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
		return fmt.Errorf("could not decode response body: %w", err)
	}

	if err = resp.Error(); err != nil {
		return err
	}

	return nil
}

func (c *Conn) ListPeople() ([]*Person, error) {
	type data struct {
		Count int `json:"Count"`
		Items []*struct {
			ID         int    `json:"Id"`
			FirstName  string `json:"FirstName"`
			LastName   string `json:"LastName"`
			EmployeeID string `json:"EmployeeID"`
			Department string `json:"Department"`
			CardNumber string `json:"CardNumber"`
		} `json:"Items"`
	}

	u := c.url()
	u.Path += "/infinias/ia/people"
	q := u.Query()
	q.Set(formKeyUsername, c.username)
	q.Set(formKeyPassword, c.password)

	var people []*Person
	total := 1
	count := 0
	for count < total {
		q.Set("Start", strconv.Itoa(count))
		u.RawQuery = q.Encode()

		r, err := http.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("could not GET people: %w", err)
		}
		defer r.Body.Close()

		resp := new(Response)
		d := new(data)
		resp.Data = d
		if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
			return nil, fmt.Errorf("could not decode response body: %w", err)
		}

		if err = resp.Error(); err != nil {
			return nil, err
		}

		for _, p := range d.Items {
			var site, card int
			if matches := cardRegexp.FindStringSubmatch(p.CardNumber); len(matches) == 3 {
				if s, err := strconv.Atoi(matches[1]); err == nil {
					site = s
				}
				if c, err := strconv.Atoi(matches[2]); err == nil {
					card = c
				}
			}

			people = append(people, &Person{
				ID:         p.ID,
				FirstName:  p.FirstName,
				LastName:   p.LastName,
				EmployeeID: p.EmployeeID,
				Department: p.Department,
				SiteCode:   site,
				CardCode:   card,
			})
		}

		total = d.Count
		count = len(people)
	}

	return people, nil
}

func (c *Conn) ListGroups() ([]*Group, error) {
	type data struct {
		Count int `json:"Count"`
		Items []*struct {
			ID   int    `json:"Id"`
			Name string `json:"Name"`
		} `json:"Items"`
	}

	u := c.url()
	u.Path += "/infinias/ia/groups"
	q := u.Query()
	q.Set(formKeyUsername, c.username)
	q.Set(formKeyPassword, c.password)

	var groups []*Group
	total := 1
	count := 0
	for count < total {
		q.Set("Start", strconv.Itoa(count))
		u.RawQuery = q.Encode()

		r, err := http.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("could not GET groups: %w", err)
		}
		defer r.Body.Close()

		resp := new(Response)
		d := new(data)
		resp.Data = d
		if err := json.NewDecoder(r.Body).Decode(resp); err != nil {
			return nil, fmt.Errorf("could not decode response body: %w", err)
		}

		if err = resp.Error(); err != nil {
			return nil, err
		}

		for _, p := range d.Items {
			groups = append(groups, &Group{
				ID:   p.ID,
				Name: p.Name,
			})
		}

		total = d.Count
		count = len(groups)
	}

	return groups, nil
}
