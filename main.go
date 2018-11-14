package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/Disconnect24/Mail-GO/patch"
	"github.com/Disconnect24/Mail-GO/utilities"
	"github.com/getsentry/raven-go"
	_ "github.com/go-sql-driver/mysql"
	"github.com/logrusorgru/aurora"
	"github.com/robfig/cron"
)

var global utilities.Config
var db *sql.DB
var salt []byte
var ravenClient *raven.Client

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse form for further usage.
		r.ParseForm()

		if global.Debug {
			log.Printf("%s %s", aurora.Blue(r.Method), aurora.Red(r.URL))
			for name, value := range r.Form {
				log.Print(name, " ", aurora.Green("=>"), " ", value)
			}
			log.Printf("Accessing from: %s", aurora.Blue(r.Host))
		}

		// Finally, serve.
		handler.ServeHTTP(w, r)
	})
}

func configHandle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		r.ParseForm()

		fileWriter, _, err := r.FormFile("uploaded_config")
		if err != nil || err == http.ErrMissingFile {
			utilities.LogError(ravenClient, "Incorrect file", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your file upload went awry. Contact our support email: support@riiconnect24.net.\nError: %v", err)
			return
		}

		file, err := ioutil.ReadAll(fileWriter)
		if err != nil {
			utilities.LogError(ravenClient, "Unable to read file", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your file upload went awry. Contact our support email support@riiconnect24.net.\nError: %v", err)
			return
		}

		patched, err := patch.ModifyNwcConfig(file, db, global, ravenClient, salt)
		if err != nil {
			utilities.LogError(ravenClient, "Unable to patch", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "It seems your patching went awry. Contact our support email: support@riiconnect24.net.\nError: %v", err)
			return
		}
		w.Header().Add("Content-Type", "application/octet-stream")
		w.Header().Add("Content-Disposition", "attachment; filename=\"nwc24msg.cfg\"")
		w.Write(patched)
		break
	case "GET":
		fmt.Fprint(w, "This page doesn't do anything by itself. Try going to the main site.")
	default:
		break
	}
}

func main() {
	log.Printf("Mail-GO Server")
	// Get salt for passwords
	saltLocation := "config/salt.bin"
	salt, err := ioutil.ReadFile(saltLocation)
	if os.IsNotExist(err) {
		log.Println("No salt found. Creating....")
		salt = make([]byte, 128)

		_, err := rand.Read(salt)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile("config/salt.bin", salt, os.ModePerm)
		if err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	// Read config
	file, err := os.Open("config/config.json")
	if err != nil {
		panic(err)
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&global)
	if err != nil {
		panic(err)
	}

	if global.Debug {
		log.Println("Connecting to MySQL...")
	}
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		global.Username, global.Password, global.Host, global.Port, global.DBName))
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	if global.RavenDSN != "" {
		ravenClient, err = raven.New(global.RavenDSN)
		if err != nil {
			panic(err)
		}
	}

	// Mail calls
	http.HandleFunc("/cgi-bin/account.cgi", Account)
	http.HandleFunc("/cgi-bin/patcher.cgi", Account)
	http.HandleFunc("/cgi-bin/check.cgi", Check)
	http.HandleFunc("/cgi-bin/receive.cgi", Receive)
	http.HandleFunc("/cgi-bin/delete.cgi", Delete)
	http.HandleFunc("/cgi-bin/send.cgi", Send)

	mailDomain = regexp.MustCompile(`w(\d{16})\@(` + global.SendGridDomain + `)`)

	// Inbound parse
	http.HandleFunc("/sendgrid/parse", sendGridHandler)

	// Site
	http.HandleFunc("/patch", configHandle)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Load from site folder
		var file []byte
		var err error
		if r.URL.Path == "/" {
			// We want index.html
			file, err = ioutil.ReadFile("./patch/site/index.html")
		} else {
			file, err = ioutil.ReadFile("./patch/site" + r.URL.Path)
		}

		// We only want existing pages.
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(file)
	})

	log.Println("Running...")
	c := cron.New()
	log.Printf("Mail-GO purges Mail older than 28 days every fortnight.")
	c.AddFunc("@every 336h", func() { purgeMail() })

	// We do this to log all access to the page.
	log.Fatal(http.ListenAndServe(global.BindTo, logRequest(http.DefaultServeMux)))
}
