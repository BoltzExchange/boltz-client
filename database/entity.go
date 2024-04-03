package database

import (
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
	row := d.QueryRow(query, id)
	return parseEntity(row)
}

func (d *Database) GetEntityByName(name string) (*Entity, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM entities WHERE name  = ?"
	row := d.QueryRow(query, name)
	return parseEntity(row)
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

func parseEntity(r row) (*Entity, error) {
	entity := &Entity{}
	err := r.Scan(&entity.Id, &entity.Name)
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
