package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Disconnect24/Mail-GO/patch"
	"github.com/Disconnect24/Mail-GO/utilities"
	"github.com/getsentry/raven-go"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jasonlvhit/gocron"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
)

var global utilities.Config
var db *sql.DB
var salt []byte
var ravenClient *raven.Client

func main() {
	log.Printf("Mail-GO Server")
	// Get salt for passwords
	saltLocation := "config/salt.bin"
	salt, err := ioutil.ReadFile(saltLocation)
	if os.IsNotExist(err) {
		log.Println("No salt found. Creating...")
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
		gin.SetMode(gin.DebugMode)
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

	// If we have Sentry support, go ahead and add it in.
	if global.RavenDSN != "" {
		ravenClient, err = raven.New(global.RavenDSN)
		if err != nil {
			panic(err)
		}
	}

	// Mail purging
	gocron.Every(2).Hours().Do(func() { purgeMail() })
	purgeMail()
	log.Printf("Mail-GO purges Mail older than 28 days every 2 hours.")

	router := gin.Default()

	// Site
	router.Use(static.Serve("/", static.LocalFile("./patch/site", false)))
	router.POST("/patch", configHandle)

	// Inbound parse
	router.POST("/sendgrid/parse", sendGridHandler)
	mailDomain = regexp.MustCompile(`w(\d{16})\@(` + global.SendGridDomain + `)`)

	// Mail calls
	v1 := router.Group("/cgi-bin")
	{
		v1.GET("/account.cgi", Account)
		v1.POST("/patcher.cgi", Account)
		v1.POST("/check.cgi", Check)
		v1.POST("/receive.cgi", Receive)
		v1.POST("/delete.cgi", Delete)
		v1.POST("/send.cgi", Send)
	}

	log.Println("Running...")
	<- gocron.Start()
	log.Println(router.Run(fmt.Sprintf(global.BindTo)))
}

func configHandle(c *gin.Context) {
	errorString := "It seems your file upload went awry. Contact our support email at support@disconnect24.xyz.\nError: %v"

	fileWriter, err := c.FormFile("uploaded_config")
	if err != nil || err == http.ErrMissingFile {
		utilities.LogError(ravenClient, "Incorrect file", err)
		c.String(http.StatusBadRequest, errorString, err)
		return
	}

	file, err := fileWriter.Open()
	if err != nil {
		utilities.LogError(ravenClient, "Unable to read file", err)
		c.String(http.StatusBadRequest, errorString, err)
		return
	}

	content, err := ioutil.ReadAll(file)
	if err != nil {
		utilities.LogError(ravenClient, "Unable to read file", err)
		c.String(http.StatusBadRequest, errorString, err)
		return
	}

	patched, err := patch.ModifyNwcConfig(content, db, global, ravenClient, salt)
	if err != nil {
		utilities.LogError(ravenClient, "Unable to patch", err)
		c.String(http.StatusInternalServerError, errorString, err)
		return
	}

	c.Header("Content-Disposition", `attachment; filename="nwc24msg.cfg"`)
	c.Data(http.StatusOK, "application/octet-stream", patched)

}
