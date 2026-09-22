package handlers

import "leiheV2/db"

var queries *db.Queries

// InitQueries wird aus der main.go aufgerufen
func InitQueries(q *db.Queries) {
	queries = q
}
