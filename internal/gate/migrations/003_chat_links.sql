CREATE TABLE IF NOT EXISTS chat_links(
 child_id TEXT PRIMARY KEY, parent_id TEXT NOT NULL,
 tenant TEXT NOT NULL, user_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS chat_links_parent ON chat_links(tenant,user_id,parent_id);
INSERT OR IGNORE INTO migrations VALUES(3);
