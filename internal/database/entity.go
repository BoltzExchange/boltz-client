package database

import (
	"fmt"
)

type Tenant struct {
	Id   Id
	Name string
}

const DefaultTenantId Id = 1
const DefaultTenantName string = "admin"

var DefaultTenant = Tenant{Id: DefaultTenantId, Name: DefaultTenantName}

func (d *Database) CreateDefaultTenant() error {
	defaultTenant, _ := d.GetTenant(DefaultTenantId)
	if defaultTenant == nil {
		if err := d.CreateTenant(&Tenant{Id: DefaultTenantId, Name: DefaultTenantName}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) CreateTenant(tenant *Tenant) error {
	query := "INSERT INTO tenants (name) VALUES (?) RETURNING id"
	row := d.QueryRow(query, tenant.Name)
	return row.Scan(&tenant.Id)
}

func (d *Database) GetTenant(id Id) (*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM tenants WHERE id = ?"
	row := d.QueryRow(query, id)
	return parseTenant(row)
}

func (d *Database) GetTenantByName(name string) (*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM tenants WHERE name  = ?"
	row := d.QueryRow(query, name)
	return parseTenant(row)
}

func (d *Database) QueryTenants() ([]*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	rows, err := d.Query("SELECT * FROM tenants")
	if err != nil {
		return nil, fmt.Errorf("failed to query tenants: %w", err)
	}
	defer rows.Close()
	var result []*Tenant
	for rows.Next() {
		tenant, err := parseTenant(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, tenant)
	}
	return result, nil
}

func parseTenant(r row) (*Tenant, error) {
	tenant := &Tenant{}
	err := r.Scan(&tenant.Id, &tenant.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tenant: %w", err)
	}
	return tenant, nil
}

func (d *Database) DeleteTenant(id int64) error {
	query := "DELETE FROM tenants WHERE id = ?"
	result, err := d.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("failed to delete tenant with id %d", id)
	}
	return nil
}
