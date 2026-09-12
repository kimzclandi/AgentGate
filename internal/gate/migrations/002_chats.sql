CREATE TABLE IF NOT EXISTS chats(
 id TEXT PRIMARY KEY, tenant TEXT NOT NULL,user_id TEXT NOT NULL,run_id TEXT NOT NULL,
 messages TEXT NOT NULL,status TEXT NOT NULL,answer TEXT NOT NULL DEFAULT '',
 pending TEXT NOT NULL DEFAULT '',rounds INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO migrations VALUES(2);
UPDATE chats SET status='interrupted' WHERE status IN ('working','awaiting_approval');
