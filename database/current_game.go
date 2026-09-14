package database

// CurrentGame tags every datastore.objects row this server creates. If this
// Postgres database is ever shared with another game server, DataStore's
// object-listing methods (SearchObject, PrepareGetObject-by-
// PersistenceTarget) must stay scoped to this value -- direct DataID
// lookups (GetObjectInfoByDataID) don't need it, since data_id is
// server-generated and globally unique, but the listing methods return
// objects the client didn't ask for by ID and would otherwise leak another
// game's rows into this one's results.
var CurrentGame = "wsc"
