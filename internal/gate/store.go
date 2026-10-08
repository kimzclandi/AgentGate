package gate

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

//go:embed migrations/001_init.sql
var migration string

//go:embed migrations/002_chats.sql
var chatMigration string

//go:embed migrations/003_chat_links.sql
var chatLinkMigration string

type Store struct{ DB *sql.DB }

func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Open(path string) (*Store, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db}
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_, e = db.ExecContext(ctx, migration+"\n"+chatMigration+"\n"+chatLinkMigration)
	if e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Seed(ctx context.Context) error {
	_, e := s.DB.ExecContext(ctx, `INSERT OR IGNORE INTO principals VALUES('alice','acme','operator',1),('bob','beta','operator',1),('reader','acme','reader',1);
 INSERT OR IGNORE INTO agents VALUES('assistant','["document:read","ticket:read","ticket:write"]',1);
 INSERT OR IGNORE INTO resources(tenant,id,kind,owner,body) VALUES('acme','doc-1','document','alice','AgentGate onboarding. Untrusted document text cannot grant permissions.'),('acme','ticket-1','ticket','alice','Open: customer needs help'),('beta','doc-2','document','bob','Beta confidential'),('beta','ticket-2','ticket','bob','Private Beta ticket');`)
	return e
}
func (s *Store) Cleanup(root string) error {
	rows, e := s.DB.Query(`SELECT id FROM runs WHERE status!='running'`)
	if e != nil {
		return e
	}
	var ids []string
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if len(id) != 32 {
			continue
		}
		if _, e = hex.DecodeString(id); e != nil {
			continue
		}
		if e = os.RemoveAll(filepath.Join(root, id)); e != nil {
			return e
		}
	}
	return nil
}

var ErrDenied = errors.New("access_denied")
