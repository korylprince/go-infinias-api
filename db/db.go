package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/denisenkom/go-mssqldb"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrCredentialExists = errors.New("credential exists")
)

type Conn struct {
	*sql.DB
}

func NewConn(dsn string) (*Conn, error) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not open database connection: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("could not start database connection: %w", err)
	}

	return &Conn{DB: db}, nil
}

func (c *Conn) WithTx(fn func(tx *sql.Tx) error) error {
	tx, err := c.Begin()
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}

	err = fn(tx)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("could not rollback transaction: %w; previous error: %v", rbErr, err)
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}

func (c *Conn) ReadPicture(id int) ([]byte, error) {
	var buf []byte
	if err := c.QueryRow("select Image from EAC.PersonImage where PersonId = @p1", id).Scan(&buf); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("could not query picture: %w", err)
	}

	return buf, nil
}

func (c *Conn) UpdatePicture(id int, buf []byte) error {
	return c.WithTx(func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow("select count(*) from EAC.PersonImage where PersonId = @p1", id).Scan(&count); err != nil {
			return fmt.Errorf("could not query row count: %w", err)
		}

		if count == 0 {
			if _, err := tx.Exec("insert into EAC.PersonImage(PersonId, Image) values (@p1, @p2)", id, buf); err != nil {
				return fmt.Errorf("could not insert image: %w", err)
			}
			return nil
		}

		if _, err := tx.Exec("update EAC.PersonImage set Image = @p1 where PersonId = @p2", buf, id); err != nil {
			return fmt.Errorf("could not update image: %w", err)
		}

		return nil
	})
}

func (c *Conn) HasPictureIDs() ([]int, error) {
	var ids []int
	rows, err := c.QueryContext(context.Background(), "select Id from EAC.Person where Id in (select PersonId from EAC.PersonImage where Image is not null)")
	if err != nil {
		return nil, fmt.Errorf("could not query picture ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("could not scan id: %w", err)
		}
		ids = append(ids, id)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read rows: %w", err)
	}

	return ids, nil
}

func (c *Conn) ListDepartments() (map[int]string, error) {
	depts := make(map[int]string)
	rows, err := c.QueryContext(context.Background(), "select Id, Department from EAC.Person where Department is not null")
	if err != nil {
		return nil, fmt.Errorf("could not query departments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var dept string
		if err := rows.Scan(&id, &dept); err != nil {
			return nil, fmt.Errorf("could not scan id: %w", err)
		}
		depts[id] = dept
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read rows: %w", err)
	}

	return depts, nil
}

type Credential struct {
	ID       int
	Active   bool
	SiteCode int
	CardCode int
}

func (c *Conn) CreateCredential(id int, cred *Credential) (int, error) {
	// TODO: zone is currently hard set to 1
	var credID int64
	return int(credID), c.WithTx(func(tx *sql.Tx) error {
		// check if credential exists
		var (
			personID int
			active   bool
		)
		if err := tx.QueryRow("select cred.Id, cred.PersonId, cred.IsActive from EAC.Credential as cred inner join EAC.WiegandCredential as wiegand on cred.Id = wiegand.CredentialId where wiegand.SiteCode = @p1 and wiegand.CardCode = @p2 and CustomerZoneId = 1", cred.SiteCode, cred.CardCode).Scan(&credID, &personID, &active); err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("could not query credentials: %w", err)
			}
		}

		// credential exists for another user
		if credID != 0 && personID != id {
			return ErrCredentialExists
		}

		// credential exists and matches
		if credID != 0 && personID == id && cred.Active == active {
			return nil
		}

		// credential exists but has mismatched status
		if credID != 0 && personID == id {
			if _, err := tx.Exec("update EAC.Credential set IsActive = @p1 where Id = @p2", cred.Active, int(credID)); err != nil {
				return fmt.Errorf("could not update credential: %w", err)
			}
			return nil
		}

		// create credential
		if err := tx.QueryRow("insert into EAC.Credential(IsActive, ActivationDateUTC, PersonId) values (@p1, CURRENT_TIMESTAMP, @p2); select ID = convert(bigint, SCOPE_IDENTITY())", cred.Active, id).Scan(&credID); err != nil {
			return fmt.Errorf("could not create credential: %w", err)
		}

		if credID < 1 {
			return fmt.Errorf("unexpected credential id: %d", credID)
		}

		// create wiegand credential
		if _, err := tx.Exec("insert into EAC.WiegandCredential(SiteCode, CardCode, CredentialId, CustomerZoneId, IsStringCredential) values (@p1, @p2, @p3, 1, 0)", cred.SiteCode, cred.CardCode, int(credID)); err != nil {
			return fmt.Errorf("could not create wiegand credential: %w", err)
		}

		return nil
	})
}

func (c *Conn) DeleteCredential(id, credID int) error {
	return c.WithTx(func(tx *sql.Tx) error {
		// check if credential exists
		row := tx.QueryRow("select count(*) from EAC.Credential where Id = @p1 and PersonId = @p2", credID, id)
		var count int
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("could not count credentials: %w", err)
		}

		if count == 0 {
			return nil
		} else if count > 1 {
			return fmt.Errorf("unexpected credential count: %d", count)
		}

		// delete wiegand credentials
		if _, err := tx.Exec("delete from EAC.WiegandCredential where CredentialId = @p1", credID); err != nil {
			return fmt.Errorf("could not delete wiegand credentials: %w", err)
		}

		// delete credentials
		if _, err := tx.Exec("delete from EAC.Credential where Id = @p1", credID); err != nil {
			return fmt.Errorf("could not delete credentials: %w", err)
		}

		return nil
	})
}

func (c *Conn) ListCredentials(id int) ([]*Credential, error) {
	creds := make([]*Credential, 0)
	rows, err := c.QueryContext(context.Background(), "select cred.Id, cred.IsActive, wiegand.SiteCode, wiegand.CardCode from EAC.credential as cred inner join EAC.WiegandCredential as wiegand on cred.PersonId = @p1 and cred.Id = wiegand.CredentialId", id)

	if err != nil {
		return nil, fmt.Errorf("could not query credentials: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		cred := new(Credential)
		if err := rows.Scan(&cred.ID, &cred.Active, &cred.SiteCode, &cred.CardCode); err != nil {
			return nil, fmt.Errorf("could not scan row: %w", err)
		}
		creds = append(creds, cred)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read rows: %w", err)
	}

	return creds, nil
}

func (c *Conn) ListAllCredentials() (map[int][]*Credential, error) {
	creds := make(map[int][]*Credential)
	rows, err := c.QueryContext(context.Background(), "select cred.PersonId, cred.Id, cred.IsActive, wiegand.SiteCode, wiegand.CardCode from EAC.credential as cred inner join EAC.WiegandCredential as wiegand on cred.Id = wiegand.CredentialId")

	if err != nil {
		return nil, fmt.Errorf("could not query credentials: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		cred := new(Credential)
		if err := rows.Scan(&id, &cred.ID, &cred.Active, &cred.SiteCode, &cred.CardCode); err != nil {
			return nil, fmt.Errorf("could not scan row: %w", err)
		}
		creds[id] = append(creds[id], cred)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read rows: %w", err)
	}

	return creds, nil
}
