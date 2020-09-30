package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	_ "github.com/lib/pq"
)

func register(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("postgres", psqlInfo)

	checkErr(err)

	err = createUserTable(db)
	checkErr(err)

	var requestBody userDB
	err = json.NewDecoder(r.Body).Decode(&requestBody)
	checkErr(err)

	err = requestBody.addNewUser(db)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		// panic(err)

		json.NewEncoder(w).Encode(map[string]string{"status": "error", "statusText": err.Error()})
	} else {
		err = requestBody.userLogin(db, w, r)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "statusText": err.Error()})
		} else {
			json.NewEncoder(w).Encode(map[string]string{"status": "success", "statusText": "Registration successful."})
		}
	}
	db.Close()
}

func homepage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "This is my personal backend.")
}

func createUserTable(db *sql.DB) error {
	crt, err := db.Prepare(`CREATE TABLE IF NOT EXISTS accounts(
		id SERIAL PRIMARY KEY,
		userID varchar(64) UNIQUE NOT NULL,
		nickname varchar(128) NOT NULL,
		portraitURI varchar(1024) NOT NULL,
		password varchar(64) NOT NULL,
		token varchar(256) UNIQUE NOT NULL,
		isAdmin bool,
		created date NOT NULL
		)`)
	checkErr(err)
	_, err = crt.Exec()
	return err
}
