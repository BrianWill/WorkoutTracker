package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	_ "github.com/lib/pq"
	uuid "github.com/satori/go.uuid"
)

const filePath = "userData.dat"

type User struct {
	Name string
	// todo how is int64 represented in JS?
	Workouts  map[int64]*Workout // indexed by StartTime
	Templates []Workout          // user's personal list of templates
	Exercises []Exercise         // user's personal list of exercises (SetsExpected used but Sets is empty)
}

type Workout struct {
	Name      string
	StartTime int64 // unix time
	EndTime   int64 // unix time
	Exercises []Exercise
}

type Exercise struct {
	Name     string
	Sets     []Set // the actual sets performed by the user
	Expected []Set // parallel (the values expected for the user to perform)
	Notes    string
}

type Set struct {
	Reps     int
	Weight   int
	Duration int // time in milliseconds of time to perform set
	Rest     int // time in milliseconds of rest before next exercise
}

type UserMap struct {
	sync.RWMutex
	internal map[string]string
}

func NewUserMap() *UserMap {
	return &UserMap{
		internal: make(map[string]string),
	}
}

func (um *UserMap) Exists(key string) bool {
	um.RLock()
	_, ok := um.internal[key]
	um.RUnlock()
	return ok
}

func (um *UserMap) Delete(key string) {
	um.Lock()
	delete(um.internal, key)
	um.Unlock()
}

func (um *UserMap) Store(userID string, data string) {
	um.Lock()
	um.internal[userID] = data
	um.Unlock()
}

func initUser() User {
	return User{}
}

func Load() (*UserMap, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	users := &UserMap{}
	return users, json.NewDecoder(f).Decode(users)
}

func Save(users *UserMap) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(users)
	if err != nil {
		return err
	}
	r := bytes.NewReader(b)
	_, err = io.Copy(f, r)
	if err != nil {
		return err
	}
	return nil
}

func main() {

	rand.Seed(time.Now().UnixNano())

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	// db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	// if err != nil {
	// 	log.Fatalf("Error opening database: %q", err)
	// }

	users, err := Load()
	if err != nil {
		users = NewUserMap()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		// wait for any pending requests to finish
		fmt.Println(" SIGINT received")s
		users.Lock()
		defer users.Unlock()
		os.Exit(0)
	}()

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl")
	router.Static("/static", "static")

	router.Static("/gojs", "gojs")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "home.tmpl", nil)
	})

	router.GET("/signup", func(c *gin.Context) {
		userID, err := c.Cookie("user_id")
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid cookie.")
		}
		users.Lock()
		defer users.Unlock()
		if _, ok := users.internal[userID]; ok {
			c.String(http.StatusOK, "you already have a user id: "+userID)
			return
		}
		u2 := uuid.NewV4()
		userID = u2.String()
		const tenYears = 10 * 365 * 24 * 60 * 60
		c.SetCookie("user_id", userID, tenYears, "/", "", false, false)
		bytes, err := json.Marshal(initUser())
		if err != nil {
			fmt.Printf("Error JSON encoding state: %+v", err)
		}
		users.internal[userID] = string(bytes)
		c.String(http.StatusOK, "new user created with id: "+userID)
	})

	// save all of user's data
	router.POST("/store", func(c *gin.Context) {
		userID, err := c.Cookie("user_id")
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid cookie.")
		}
		payload := c.PostForm("payload")

		// verify the data is valid
		var data *User
		err = json.Unmarshal([]byte(payload), data)
		if err != nil {
			fmt.Printf("error decoding JSON: %s", err)
			c.String(http.StatusBadRequest, "Error in decoding JSON payload.")
			return
		}

		users.Lock()
		defer users.Unlock()
		_, ok := users.internal[userID]
		if !ok {
			c.String(http.StatusUnauthorized, "You are not a known user.")
			users.Unlock()
			return
		}
		users.internal[userID] = payload

		err = Save(users)
		if err != nil {
			fmt.Println("error saving user map: " + err.Error())
		}

		c.String(http.StatusOK, "Saved data.")
	})

	// get all of user's data
	router.POST("/load", func(c *gin.Context) {
		userID, err := c.Cookie("user_id")
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid cookie.")
		}
		users.Lock()
		defer users.Unlock()
		data, ok := users.internal[userID]
		if !ok {
			c.String(http.StatusUnauthorized, "You are not a known user.")
			return
		}
		c.JSON(http.StatusOK, data)
	})

	router.Run(":" + port)
}
