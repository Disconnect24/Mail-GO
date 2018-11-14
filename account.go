package main

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Disconnect24/Mail-GO/utilities"
	_ "github.com/go-sql-driver/mysql"
	"github.com/logrusorgru/aurora"
)

func Account(w http.ResponseWriter, r *http.Request) {
	var is string
	// Check if we should use `=` for a Wii or
	// `:` for the Homebrew patcher.
	if r.URL.Path == "/cgi-bin/account.cgi" {
		is = "="
	} else {
		is = ":"
	}

	wiiID := r.Form.Get("mlid")
	if !utilities.FriendCodeIsValid(wiiID) {
		fmt.Fprint(w, GenAccountErrorCode(610, is, "Invalid Wii Friend Code."))
		return
	}

	w.Header().Add("Content-Type", "text/plain;charset=utf-8")

	stmt, err := db.Prepare("INSERT IGNORE INTO `accounts` (`mlid`,`passwd`, `mlchkid` ) VALUES (?, ?, ?)")
	if err != nil {
		fmt.Fprint(w, GenAccountErrorCode(410, is, "Database error."))
		utilities.LogError(ravenClient, "Unable to prepare account statement", err)
		return
	}

	passwd := utilities.RandStringBytesMaskImprSrc(16)
	passwdByte := sha512.Sum512(append(salt, []byte(passwd)...))
	passwdHash := hex.EncodeToString(passwdByte[:])

	mlchkid := utilities.RandStringBytesMaskImprSrc(32)
	mlchkidByte := sha512.Sum512(append(salt, []byte(mlchkid)...))
	mlchkidHash := hex.EncodeToString(mlchkidByte[:])

	result, err := stmt.Exec(wiiID, passwdHash, mlchkidHash)
	if err != nil {
		fmt.Fprint(w, GenAccountErrorCode(410, is, "Database error."))
		utilities.LogError(ravenClient, "Unable to execute statement", err)
		return
	}

	affected, err := result.RowsAffected()
	if err != nil {
		fmt.Fprint(w, GenAccountErrorCode(410, is, "Database error."))
		utilities.LogError(ravenClient, "Unable to get rows affected", err)
		return
	}

	if affected == 0 {
		fmt.Fprint(w, GenAccountErrorCode(211, is, "Duplicate registration."))
		return
	}

	fmt.Fprint(w, utilities.GenSuccessResponseTyped(is),
		"mlid", is, wiiID, "\n",
		"passwd", is, passwd, "\n",
		"mlchkid", is, mlchkid, "\n")
}

func GenAccountErrorCode(error int, is string, reason string) string {
	log.Println(aurora.Red("[Warning]"), "Encountered error", error, "with reason", reason)

	return fmt.Sprint(
		"cd", is, strconv.Itoa(error), "\n",
		"msg", is, reason, "\n")
}
