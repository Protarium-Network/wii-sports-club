package database

import (
	"os"

	"github.com/Protarium-Network/wii-sports-club/globals"
)

func initPostgres() {
	var err error

	_, err = Postgres.Exec(`CREATE TABLE IF NOT EXISTS rankings (
		owner_pid bigint,
		unique_id bigint,
		category bigint,
		score bigint,
		order_by smallint,
		update_mode smallint,
		groups bytea,
		param bigint,
		updated_at bigint,
		PRIMARY KEY (unique_id, owner_pid, category)
	)`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	// Older revisions used signed 32-bit columns for PIDs and scores even
	// though NEX uses unsigned 32-bit values. Widen them in-place so existing
	// installations are migrated without losing leaderboard data.
	_, err = Postgres.Exec(`
		ALTER TABLE rankings
			ALTER COLUMN owner_pid TYPE bigint USING owner_pid::bigint,
			ALTER COLUMN category TYPE bigint USING category::bigint,
			ALTER COLUMN score TYPE bigint USING score::bigint,
			ALTER COLUMN order_by TYPE smallint USING order_by::smallint,
			ALTER COLUMN update_mode TYPE smallint USING update_mode::smallint
	`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`CREATE TABLE IF NOT EXISTS common_datas (
		unique_id bigint,
		owner_pid bigint,
		common_data bytea,
		updated_at bigint,
		PRIMARY KEY (unique_id, owner_pid)
	)`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`
		ALTER TABLE common_datas
			ALTER COLUMN owner_pid TYPE bigint USING owner_pid::bigint
	`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`CREATE TABLE IF NOT EXISTS ranking_categories (
		category bigint PRIMARY KEY,
		order_by smallint NOT NULL CHECK (order_by IN (0, 1)),
		created_at bigint NOT NULL
	)`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	// Preserve the direction of categories uploaded before the
	// ranking_categories table was introduced.
	_, err = Postgres.Exec(`
		INSERT INTO ranking_categories (category, order_by, created_at)
		SELECT category, COALESCE(MIN(order_by), 1), COALESCE(MIN(updated_at), 0)
		FROM rankings
		GROUP BY category
		ON CONFLICT (category) DO NOTHING
	`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`
		CREATE INDEX IF NOT EXISTS rankings_category_score_idx
			ON rankings (category, score, updated_at);
		CREATE INDEX IF NOT EXISTS rankings_owner_category_idx
			ON rankings (owner_pid, category);
		CREATE INDEX IF NOT EXISTS common_datas_owner_updated_idx
			ON common_datas (owner_pid, updated_at DESC)
	`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`CREATE SCHEMA IF NOT EXISTS datastore`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	globals.Logger.Success("datastore Postgres schema created")

	_, err = Postgres.Exec(`CREATE SEQUENCE IF NOT EXISTS datastore.object_data_id_seq
		INCREMENT 1
		MINVALUE 1
		MAXVALUE 281474976710656
		START 1
		CACHE 1`, // * Honestly I don't know what CACHE does but I saw it recommended so here it is
	)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	_, err = Postgres.Exec(`CREATE TABLE IF NOT EXISTS datastore.objects (
			data_id bigint NOT NULL DEFAULT nextval('datastore.object_data_id_seq') PRIMARY KEY,
			upload_completed boolean NOT NULL DEFAULT FALSE,
			deleted boolean NOT NULL DEFAULT FALSE,
			owner bigint,
			size int,
			name text,
			data_type int,
			meta_binary bytea,
			permission int,
			permission_recipients int[],
			delete_permission int,
			delete_permission_recipients int[],
			flag int,
			period int,
			refer_data_id bigint,
			tags text[],
			persistence_slot_id int,
			extra_data text[],
			access_password bigint NOT NULL DEFAULT 0,
			update_password bigint NOT NULL DEFAULT 0,
			creation_date timestamp,
			update_date timestamp
		)`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	// game scopes DataStore::SearchObject / PrepareGetObject-by-PersistenceTarget
	// results to the game that created each object -- see database.CurrentGame
	// doc comment for why this exists. Backfill runs once (idempotent: no more
	// NULL rows after the first pass on any binary).
	_, err = Postgres.Exec(`ALTER TABLE datastore.objects ADD COLUMN IF NOT EXISTS game text`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}
	_, err = Postgres.Exec(`UPDATE datastore.objects SET game = 'wsc' WHERE game IS NULL`)
	if err != nil {
		globals.Logger.Critical(err.Error())
		os.Exit(1)
	}

	globals.Logger.Success("Postgres tables created")
}
