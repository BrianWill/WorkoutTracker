package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	_ "github.com/heroku/x/hmetrics/onload"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"upper.io/db.v3/lib/sqlbuilder"
	"upper.io/db.v3/postgresql"
	"upper.io/db.v3/sqlite"
)

const sqliteFilePath = "userData.dat"

// type User struct {
// 	Name     string
// 	Password string
// 	// todo how is int64 represented in JS?
// 	Workouts  map[int64]*Workout // indexed by StartTime
// 	Templates []Workout          // user's personal list of templates
// 	Exercises []Exercise         // user's personal list of exercises (SetsExpected used but Sets is empty)
// }

// type Workout struct {
// 	Name      string
// 	StartTime int64 // unix time
// 	EndTime   int64 // unix time
// 	Exercises []Exercise
// }

// type Exercise struct {
// 	Name     string
// 	Sets     []Set // the actual sets performed by the user
// 	Expected []Set // parallel (the values expected for the user to perform)
// 	Notes    string
// }

// type Set struct {
// 	Reps     int
// 	Weight   int
// 	Duration int // time in milliseconds of time to perform set
// 	Rest     int // time in milliseconds of rest before next exercise
// }

func initPostgres(db sqlbuilder.Database) error {

	return nil
}

type UserDB struct {
	ID       uint64 `db:"id,omitempty"`
	Name     string `db:"name"`
	Password string `db:"password"`
}

type ExerciseDB struct {
	ID    uint64 `db:"id,omitempty"`
	Name  string `db:"name"`
	Notes string `db:"notes"`
}

func initSqlite(db sqlbuilder.Database) error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			password TEXT NOT NULL
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS exercises(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			notes TEXT NOT NULL
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS workouts(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			startTime INTEGER NOT NULL,
			endTime INTEGER NOT NULL
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
			exercise INTEGER NOT NULL,
			workout INTEGER NOT NULL,
			FOREIGN KEY (exercise) REFERENCES exercises(id),
			FOREIGN KEY (workout) REFERENCES workouts(id)
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS workout_exercises(
			id INTEGER PRIMARY KEY,
			exercise INTEGER NOT NULL,
			workout INTEGER NOT NULL,
			FOREIGN KEY (exercise) REFERENCES exercises(id),
			FOREIGN KEY (workout) REFERENCES workouts(id)
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
	dev := os.Getenv("DEV") == "1"

	var err error
	var db sqlbuilder.Database
	if dev {
		fmt.Println("DEV MODE")
		db, err = sqlite.Open(sqlite.ConnectionURL{sqliteFilePath, nil})
	} else {
		fmt.Println("PRODUCTION MODE")
		connURL, connErr := postgresql.ParseURL(os.Getenv("DATABASE_URL"))
		if connErr != nil {
			log.Fatalf("Error with Postgres connection string: %q", err)
		}
		db, err = postgresql.Open(connURL)
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
		log.Fatalf("Error initializing database: %s", err)
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

	router.GET("/admin/users", func(c *gin.Context) {
		var users []UserDB
		err := db.Collection("users").Find().All(&users)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading users. "+err.Error())
			return
		}
		c.HTML(http.StatusOK, "admin_users.tmpl", users)
	})

	router.GET("/admin/exercises", func(c *gin.Context) {
		var exercises []ExerciseDB
		err := db.Collection("exercises").Find().All(&exercises)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading exercises. "+err.Error())
			return
		}
		c.HTML(http.StatusOK, "admin_exercises.tmpl", exercises)
	})

	router.POST("/json/addUser", func(c *gin.Context) {
		buf := &bytes.Buffer{}
		buf.ReadFrom(c.Request.Body)
		user := UserDB{
			Name:     buf.String(),
			Password: "",
		}
		_, err := db.Collection("users").Insert(user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't add new user."+err.Error())
			return
		}
		c.String(http.StatusOK, user.Name)
	})

	router.POST("/json/removeUser", func(c *gin.Context) {
		buf := &bytes.Buffer{}
		buf.ReadFrom(c.Request.Body)
		s := buf.String()
		userID, err := strconv.Atoi(s)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid id for user to remove. "+err.Error())
			return
		}
		err = db.Collection("users").Find(userID).Delete()
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't remove user. "+err.Error())
			return
		}
		c.String(http.StatusOK, "removed user with id: "+s)
	})

	router.POST("/json/addExercise", func(c *gin.Context) {
		var exercise ExerciseDB
		c.MustBindWith(&exercise, binding.JSON)
		_, err := db.Collection("exercises").Insert(exercise)
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't add new exercise."+err.Error())
			return
		}
		c.String(http.StatusOK, exercise.Name)
	})

	router.POST("/json/removeExercise", func(c *gin.Context) {
		buf := &bytes.Buffer{}
		buf.ReadFrom(c.Request.Body)
		s := buf.String()
		exerciseID, err := strconv.Atoi(s)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid id for exercise to remove. "+err.Error())
			return
		}
		err = db.Collection("exercises").Find(exerciseID).Delete()
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't remove exercise. "+err.Error())
			return
		}
		c.String(http.StatusOK, "removed exercise with id: "+s)
	})

	router.GET("/dev/exercises", func(c *gin.Context) {
		// show all exercises
		// page has form to add a new exercise
		c.HTML(http.StatusOK, "exercises.tmpl", nil)
	})

	router.GET("/dev/workouts", func(c *gin.Context) {
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

	router.Run(":" + port)
}
