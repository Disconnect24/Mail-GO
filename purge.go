package main

import (
	"log"
)

// Intuitive text to remind you that Mail-GO has a purging feature.
// A feature as simple as this has caused a lot of commotion.
// But fear begone, as the mailman no longer has to carry old and grotty mail.

func purgeMail() {
	log.Printf("Mail-GO will now optimise the mail tables." +
		"This may take a little while, and some interruptions may occur." +
		"PURGING MAIL...")
	// BEGONE MAIL!
	stmtIns, err := db.Prepare("DELETE FROM WC24Mail.mails WHERE `timestamp` < NOW() - INTERVAL 28 DAY;")
	if err != nil {
		log.Panicf("Failed to prepare purge statement: %v", err)
	}
	_, err = stmtIns.Exec()
	if err != nil {
		log.Panicf("Failed to execute purge statement: %v", err)
	}
}
