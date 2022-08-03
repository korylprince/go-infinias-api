package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/denisenkom/go-mssqldb"
)

var ErrNotFound = errors.New("not found")

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

func (c *Conn) ReadDepartments() (map[int]string, error) {
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
