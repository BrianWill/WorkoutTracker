package main

import (
	"database/sql"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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

func initPostgres(db *sql.DB) error {

	return nil
}

func initSqlite(db *sql.DB) error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY,
   			name TEXT NOT NULL,
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS sets(
			id INTEGER PRIMARY KEY,
			reps     		 INTEGER NOT NULL,
			weight   		 INTEGER NOT NULL,
			duration 		 INTEGER NOT NULL,
			rest     		 INTEGER NOT NULL,
			repsExpected     INTEGER NOT NULL,
			weightExpected   INTEGER NOT NULL,
			durationExpected INTEGER NOT NULL,
			restExpected     INTEGER NOT NULL,
			FOREIGN KEY (exercise) REFERENCES exercises(id),
			FOREIGN KEY (workout) REFERENCES workouts(id),
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS exercises(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			notes TEXT NOT NULL,
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS workouts(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			startTime INTEGER NOT NULL,
			endTime INTEGER NOT NULL,
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS workout_exercises(
			id INTEGER PRIMARY KEY,
			exercise INTEGER NOT NULL,
			workout INTEGER NOT NULL,
			FOREIGN KEY (exercise) REFERENCES exercises(id),
			FOREIGN KEY (workout) REFERENCES workouts(id),
		)`); err != nil {
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
	dev := os.Getenv("DEV") == "true"

	var db *sql.DB
	var err error
	if dev {
		db, err = sql.Open("sqlite3", "./dev.db")
	} else {
		db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	}
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}
	defer db.Close()
	if dev {
		err = initSqlite(db)
	} else {
		err = initPostgres(db)
	}
	if err != nil {
		log.Fatalf("Error initializing database: %q", err)
	}

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

		//// check in DB if user already exists
		// users.Lock()
		// defer users.Unlock()
		// if _, ok := users.internal[userID]; ok {
		// 	c.String(http.StatusOK, "you already have a user id: "+userID)
		// 	return
		// }
		// u2 := uuid.NewV4()
		// userID = u2.String()
		// const tenYears = 10 * 365 * 24 * 60 * 60
		// c.SetCookie("user_id", userID, tenYears, "/", "", false, false)
		// bytes, err := json.Marshal(initUser())
		// if err != nil {
		// 	fmt.Printf("Error JSON encoding state: %+v", err)
		// }
		// users.internal[userID] = string(bytes)
		c.String(http.StatusOK, "new user created with id: "+userID)
	})

	router.GET("/dev/exercises", func(c *gin.Context) {
		// show all exercises
		// page has form to add a new exercise
		c.HTML(http.StatusOK, "exercises.tmpl", nil)
	})

	router.GET("/dev/exercises", func(c *gin.Context) {
		// show all workouts
		// page has form to add a new workout (presents lists of exercises to add)
		// forms to delete/edit a workout
		c.HTML(http.StatusOK, "workouts.tmpl", nil)
	})

	router.POST("/dev/addExercise", func(c *gin.Context) {

		c.String(http.StatusOK, "Added exercise.")
	})

	router.POST("/dev/addWorkout", func(c *gin.Context) {

		c.String(http.StatusOK, "Added workout.")
	})

	// // save all of user's data
	// router.POST("/store", func(c *gin.Context) {
	// 	userID, err := c.Cookie("user_id")
	// 	if err != nil {
	// 		c.String(http.StatusBadRequest, "Invalid cookie.")
	// 	}

	// 	users.Lock()
	// 	defer users.Unlock()
	// 	_, ok := users.internal[userID]
	// 	if !ok {
	// 		c.String(http.StatusUnauthorized, "You are not a known user.")
	// 		users.Unlock()
	// 		return
	// 	}

	// 	buf := bytes.NewBuffer(nil)
	// 	io.Copy(buf, c.Request.Body)
	// 	users.internal[userID] = string(buf.Bytes())

	// 	err = Save(users)
	// 	if err != nil {
	// 		fmt.Println("error saving user map: " + err.Error())
	// 	}

	// 	c.String(http.StatusOK, "Saved data.")
	// })

	// // get all of user's data
	// router.GET("/load", func(c *gin.Context) {
	// 	userID, err := c.Cookie("user_id")
	// 	if err != nil {
	// 		c.String(http.StatusBadRequest, "Invalid cookie.")
	// 	}
	// 	users.Lock()
	// 	defer users.Unlock()
	// 	data, ok := users.internal[userID]
	// 	if !ok {
	// 		c.String(http.StatusUnauthorized, "You are not a known user.")
	// 		return
	// 	}
	// 	c.PureJSON(http.StatusOK, data)
	// })

	router.Run(":" + port)
}

// func Load() (*UserMap, error) {
// 	f, err := os.Open(filePath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer f.Close()
// 	users := &UserMap{}
// 	return users, json.NewDecoder(f).Decode(users)
// }

// func Save(users *UserMap) error {
// 	f, err := os.Create(filePath)
// 	if err != nil {
// 		return err
// 	}
// 	defer f.Close()
// 	b, err := json.Marshal(users)
// 	if err != nil {
// 		return err
// 	}
// 	r := bytes.NewReader(b)
// 	_, err = io.Copy(f, r)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }
