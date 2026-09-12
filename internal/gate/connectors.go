package gate

import (
	"context"
	"database/sql"
)

// Connectors are trusted in-process adapters, never model-supplied code. These
// local adapters share the authorization transaction; remote adapters need a
// separate outbox/reconciliation design and cannot reuse this atomicity claim.
type Resource struct{ Body, Owner, Kind string }
type connector interface {
	Load(context.Context, *sql.Tx, string, string) (Resource, error)
	Apply(context.Context, *sql.Tx, Identity, Params) error
}
type localConnector struct {
	kind  string
	write bool
}

func connectorFor(kind string) (connector, error) {
	switch kind {
	case "document":
		return localConnector{kind: "document"}, nil
	case "ticket":
		return localConnector{kind: "ticket", write: true}, nil
	default:
		return nil, ErrDenied
	}
}
func (c localConnector) Load(ctx context.Context, tx *sql.Tx, tenant, id string) (Resource, error) {
	var r Resource
	e := tx.QueryRowContext(ctx, `SELECT body,owner,kind FROM resources WHERE tenant=? AND id=? AND kind=?`, tenant, id, c.kind).Scan(&r.Body, &r.Owner, &r.Kind)
	return r, e
}
func (c localConnector) Apply(ctx context.Context, tx *sql.Tx, i Identity, p Params) error {
	if !c.write {
		return ErrDenied
	}
	res, e := tx.ExecContext(ctx, `UPDATE resources SET body=?,version=version+1 WHERE tenant=? AND id=? AND owner=? AND kind=?`, p.Body, i.Tenant, p.ResourceID, i.User, c.kind)
	if e != nil {
		return e
	}
	n, e := res.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrDenied
	}
	return nil
}
