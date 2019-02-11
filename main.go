package main

import (
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/heroku/x/hmetrics/onload"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	s := ""
	if h > 0 {
		s += strconv.Itoa(int(h)) + " hrs "
	}
	s += strconv.Itoa(int(m)) + " min"
	return s
}

func main() {
	rand.Seed(time.Now().UnixNano())

	//port := os.Getenv("PORT")
	port := "5001"

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.tmpl", nil)
	})

	router.Run(":" + port)
}
