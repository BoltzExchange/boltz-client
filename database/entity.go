package database

import (
	"database/sql"
	"fmt"
)

type Entity struct {
	Id   int64
	Name string
}

func (d *Database) CreateEntity(entity *Entity) error {
	query := "INSERT INTO entities (name) VALUES (?) RETURNING id"
	row := d.QueryRow(query, entity.Name)
	return row.Scan(&entity.Id)
}

func (d *Database) GetEntity(id int64) (*Entity, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM entities WHERE id = ?"
	rows, err := d.Query(query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return parseEntity(rows)
	}
	return nil, fmt.Errorf("could not find entity with id %d", id)
}

func (d *Database) GetEntityByName(name string) (*Entity, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM entities WHERE name  = ?"
	rows, err := d.Query(query, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return parseEntity(rows)
	}
	return nil, fmt.Errorf("could not find entity with name %s", name)
}

func (d *Database) QueryEntities() ([]*Entity, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	rows, err := d.Query("SELECT * FROM entities")
	if err != nil {
		return nil, fmt.Errorf("failed to query entities: %w", err)
	}
	defer rows.Close()
	var result []*Entity
	for rows.Next() {
		entity, err := parseEntity(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

func parseEntity(rows *sql.Rows) (*Entity, error) {
	entity := &Entity{}
	err := rows.Scan(&entity.Id, &entity.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse entity: %w", err)
	}
	return entity, nil
}

func (d *Database) DeleteEntity(id int64) error {
	query := "DELETE FROM entities WHERE id = ?"
	result, err := d.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("failed to delete entity with id %d", id)
	}
	return nil
}
