package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	appData, _ := os.UserConfigDir()
	dbPath := filepath.Join(appData, "TTI", "BridgeGround", "api", "bridgeground.db")

	fmt.Printf("Opening DB at: %s\n", dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Check table info
	rows, err := db.Query("PRAGMA table_info(shops)")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fmt.Println("--- Schema ---")
	for rows.Next() {
		var cid int
		var name, typeStr string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err != nil {
			panic(err)
		}
		fmt.Printf("Column: %s (%s)\n", name, typeStr)
	}

	// Check one row
	// row := db.QueryRow("SELECT * FROM shops LIMIT 1")
	// We don't know columns count easily with QueryRow without preparing, so let's use Query
	rows2, err := db.Query("SELECT * FROM shops LIMIT 1")
	if err != nil {
		panic(err)
	}
	defer rows2.Close()

	cols, _ := rows2.Columns()
	fmt.Printf("\n--- Columns in SELECT * ---: %v\n", cols)
	
	if rows2.Next() {
		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}
		rows2.Scan(valPtrs...)
		fmt.Println("--- First Row Data ---")
		for i, col := range cols {
			fmt.Printf("%s: %v\n", col, vals[i])
		}
	} else {
		fmt.Println("No data in shops table.")
	}
}

