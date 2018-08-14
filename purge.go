package main

import (
	"database/sql"
	"log"
)

func purgeMail(db *sql.DB) {
	log.Printf("Mail-Go will now optimise the mail tables." +
		"This may take a little while, and some interruptions may occur." +
		"PURGING MAIL...")
	//	Prepare response.
	db.Exec("DELETE FROM WC24Mail.mails WHERE `timestamp` < NOW() - INTERVAL 28 DAY;")
}
