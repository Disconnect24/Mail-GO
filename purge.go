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
	if err != nil { // In case the statement couldn't be prepared.
		log.Printf("Oops. Mail-GO could not purge/optimise mail." +
			"You may want to check the database or program as there may be other issues." +
			"If you need assistance, visit Disconnect24's Discord server." +
			"(Failed at preparing statement.)")
		panic(err.Error())
	}
	_, err = stmtIns.Exec()
	if err != nil { // In case the statement couldn't be executed.
		log.Printf("Oops. Mail-GO could not purge/optimise mail." +
			"You may want to check the database or program as there may be other issues." +
			"If you need assistance, visit Disconnect24's Discord server." +
			"(Failed at executing statement.)")
		panic(err.Error())
	}
}
