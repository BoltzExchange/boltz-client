package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// Tenant represents a tenant in the multi-tenant system.
// Each tenant can have its own wallets, swaps, and autoswap configuration.
type Tenant struct {
	Id   Id
	Name string
}

const (
	// DefaultTenantId is the ID of the default admin tenant.
	DefaultTenantId Id = 1
	// DefaultTenantName is the name of the default admin tenant.
	DefaultTenantName string = "admin"
)

// DefaultTenant is the default admin tenant that is created on first startup.
var DefaultTenant = Tenant{Id: DefaultTenantId, Name: DefaultTenantName}

// CreateDefaultTenant creates the default admin tenant if it doesn't exist.
func (d *Database) CreateDefaultTenant() error {
	defaultTenant, _ := d.GetTenant(DefaultTenantId)
	if defaultTenant == nil {
		if err := d.CreateTenant(&Tenant{Id: DefaultTenantId, Name: DefaultTenantName}); err != nil {
			return err
		}
	}
	return nil
}

// CreateTenant creates a new tenant in the database.
// The tenant ID is automatically generated and assigned.
func (d *Database) CreateTenant(tenant *Tenant) error {
	query := "INSERT INTO tenants (name) VALUES (?) RETURNING id"
	row := d.QueryRow(query, tenant.Name)
	return row.Scan(&tenant.Id)
}

// GetTenant retrieves a tenant by its ID.
func (d *Database) GetTenant(id Id) (*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM tenants WHERE id = ?"
	row := d.QueryRow(query, id)
	return parseTenant(row)
}

// GetTenantByName retrieves a tenant by its name.
func (d *Database) GetTenantByName(name string) (*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT * FROM tenants WHERE name  = ?"
	row := d.QueryRow(query, name)
	return parseTenant(row)
}

// QueryTenants retrieves all tenants from the database.
func (d *Database) QueryTenants() ([]*Tenant, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	rows, err := d.Query("SELECT * FROM tenants")
	if err != nil {
		return nil, fmt.Errorf("failed to query tenants: %w", err)
	}
	defer closeRows(rows)
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

// parseTenant parses a database row into a Tenant struct.
func parseTenant(r row) (*Tenant, error) {
	tenant := &Tenant{}
	err := r.Scan(&tenant.Id, &tenant.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tenant: %w", err)
	}
	return tenant, nil
}

// DeleteTenant removes a tenant from the database by its ID.
// Returns an error if the tenant doesn't exist or deletion fails.
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

// HasTenantWallets checks if a tenant has any associated wallets.
func (d *Database) HasTenantWallets(tenantId Id) (bool, error) {
	d.lock.RLock()
	defer d.lock.RUnlock()
	query := "SELECT 1 FROM wallets WHERE tenantId = ? LIMIT 1"
	var exists int
	err := d.QueryRow(query, tenantId).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check tenant wallets: %w", err)
	}
	return true, nil
}
