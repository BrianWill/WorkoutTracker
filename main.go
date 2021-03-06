package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	_ "github.com/heroku/x/hmetrics/onload"

	uuid "github.com/satori/go.uuid"
	up "upper.io/db.v3"
	"upper.io/db.v3/lib/sqlbuilder"
	"upper.io/db.v3/postgresql"
	"upper.io/db.v3/sqlite"
)

const sqliteFilePath = "userData.dat"
const timeFormat = "15:04 Mon _2 Jan 2006"

func initPostgres(db sqlbuilder.Database) error {

	return nil
}

type UserDB struct {
	ID       uint64 `db:"id,omitempty"`
	Name     string `db:"name"`
	Cookie   string `db:"cookie"`
	Password string `db:"password"`
}

type ExerciseDB struct {
	ID    uint64 `db:"id,omitempty"`
	Name  string `db:"name" json:"name"`
	Notes string `db:"notes" json:"notes"`
}

type Exercise struct {
	ExerciseDB
	Sets []SetDB
}

type WorkoutDB struct {
	ID           uint64 `db:"id,omitempty"`
	Name         string `db:"name" json:"name"`
	StartTime    uint64 `db:"startTime" json:"startTime"`
	StartTimeStr string
	EndTime      uint64 `db:"endTime" json:"endTime"`
	User         uint64 `db:"user" json:"user"`
}

type Workout struct {
	WorkoutDB
	Exercises []Exercise
}

type SetDB struct {
	ID               uint64 `db:"id,omitempty"`
	Order            int    `db:"order"` // sets of an exercise have a relative order
	Reps             int    `db:"reps"`
	Weight           int    `db:"weight"`
	Duration         int    `db:"duration"` // time in milliseconds of time to perform set
	Rest             int    `db:"rest"`     // time in milliseconds of rest before next exercise
	RepsExpected     int    `db:"repsExpected"`
	WeightExpected   int    `db:"weightExpected"`
	DurationExpected int    `db:"durationExpected"` // time in milliseconds of time to perform set
	RestExpected     int    `db:"restExpected"`     // time in milliseconds of rest before next exercise
}

func initSqlite(db sqlbuilder.Database) error {
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			cookie TEXT NOT NULL,
			password TEXT NOT NULL
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS exercises(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			notes TEXT NOT NULL,
			workout INTEGER NOT NULL,
			FOREIGN KEY (workout) REFERENCES workouts(id)
		)`); err != nil {
		return err
	}

	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS workouts(
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			startTime INTEGER NOT NULL,
			endTime INTEGER NOT NULL,
			user INTEGER NOT NULL,
			FOREIGN KEY (user) REFERENCES users(id)
		)`); err != nil {
		return err
	}

	//
	if _, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS sets(
			id INTEGER PRIMARY KEY,
			order            INTEGER NOT NULL,    /* first is 0, second is 1, etc. */
			reps     		 INTEGER NOT NULL,
			weight   		 INTEGER NOT NULL,
			duration 		 INTEGER NOT NULL,
			rest     		 INTEGER NOT NULL,
			repsExpected     INTEGER NOT NULL,
			weightExpected   INTEGER NOT NULL,
			durationExpected INTEGER NOT NULL,
			restExpected     INTEGER NOT NULL,
			exercise INTEGER NOT NULL,
			FOREIGN KEY (exercise) REFERENCES exercises(id),
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
		userCookie, err := c.Cookie("user_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		user := UserDB{}
		err = db.Collection("users").Find(up.Cond{"cookie": userCookie}).One(&user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user info. "+err.Error())
			return
		}
		workouts := []WorkoutDB{}
		err = db.Collection("workouts").Find(up.Cond{"user": user.ID}).All(&workouts)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user workouts. "+err.Error())
			return
		}
		sort.Slice(workouts, func(i, j int) bool {
			return workouts[i].StartTime < workouts[j].StartTime
		})
		for i, v := range workouts {
			workouts[i].StartTimeStr = time.Unix(int64(v.StartTime), 0).Format(timeFormat)
		}
		c.HTML(http.StatusOK, "home.tmpl", workouts)
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.tmpl", nil)
	})

	router.POST("/login", func(c *gin.Context) {
		// todo: put in transaction

		name := c.PostForm("username")
		password := c.PostForm("password")
		var user UserDB
		err := db.Collection("users").Find(up.Cond{"name": name, "password": password}).One(&user)
		if err != nil {
			c.String(http.StatusUnauthorized, "Bad user name and/or password.")
			return
		}

		u2 := uuid.NewV4()
		userID := u2.String()
		const tenYears = 10 * 365 * 24 * 60 * 60
		c.SetCookie("user_id", userID, tenYears, "/", "", false, false)

		user.Cookie = userID
		err = db.Collection("users").Find(user.ID).Update(user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Bad user name and/or password.")
			return
		}
		c.Redirect(http.StatusSeeOther, "/")
	})

	router.POST("/createAccount", func(c *gin.Context) {
		name := c.PostForm("username")
		password := c.PostForm("password")

		// todo: use transaction; verify that name and password are valid
		fmt.Println("create account with name & password: ", name, password)

		u2 := uuid.NewV4()
		userID := u2.String()
		const tenYears = 10 * 365 * 24 * 60 * 60
		c.SetCookie("user_id", userID, tenYears, "/", "", false, false)

		_, err := db.Collection("users").Insert(UserDB{
			Name:     name,
			Password: password,
			Cookie:   userID,
		})
		if err != nil {
			c.String(http.StatusInternalServerError, "Error creating new user. "+err.Error())
			return
		}

		c.Redirect(http.StatusSeeOther, "/")
	})

	router.GET("/createWorkout", func(c *gin.Context) {
		userCookie, err := c.Cookie("user_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		user := UserDB{}
		err = db.Collection("users").Find(up.Cond{"cookie": userCookie}).One(&user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user info. "+err.Error())
			return
		}
		workout := WorkoutDB{
			Name:      "new session",
			User:      user.ID,
			StartTime: uint64(time.Now().Unix()),
		}
		_, err = db.Collection("workouts").Insert(workout)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error creating new workout session. "+err.Error())
			return
		}
		c.Redirect(http.StatusSeeOther, "/")
	})

	router.GET("/createWorkout/:id", func(c *gin.Context) {
		workoutID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid workout ID.")
			return
		}
		userCookie, err := c.Cookie("user_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		user := UserDB{}
		err = db.Collection("users").Find(up.Cond{"cookie": userCookie}).One(&user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user info. "+err.Error())
			return
		}
		var workout WorkoutDB
		err = db.Collection("workouts").Find(workoutID).One(&workout)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error deleting workout session. "+err.Error())
			return
		}
		workout.ID = 0 // must be zero for auto-increment ID
		workout.StartTime = uint64(time.Now().Unix())
		// todo: copy all exercises associated with the workout
		_, err = db.Collection("workouts").Insert(workout)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error creating new workout session. "+err.Error())
			return
		}
		c.Redirect(http.StatusSeeOther, "/")
	})

	router.GET("/workout/:id", func(c *gin.Context) {
		workoutID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid workout ID.")
			return
		}
		userCookie, err := c.Cookie("user_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		user := UserDB{}
		err = db.Collection("users").Find(up.Cond{"cookie": userCookie}).One(&user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user info. "+err.Error())
			return
		}

		q := db.Select("w.id AS workout_id", "e.id AS exercise_id", "s.id AS set_id", "*").
			From("workouts AS w").Where("w.user = ?", user.ID).And("w.id = ?", workoutID).
			Join("exercises AS e").On("e.workout = w.id").
			Join("sets AS s").On("s.exercise = e.id").
			OrderBy("w.id", "e.id", "s.order")
		row := struct {
			WorkoutID    uint64 `db:"workout_id"`
			ExerciseID   uint64 `db:"exercise_id"`
			SetID        uint64 `db:"set_id"`
			WorkoutName  string `db:"workout_name"`
			ExerciseName string `db:"exercise_name"`
			WorkoutDB    `db:",inline"`
			ExerciseDB   `db:",inline"`
			SetDB        `db:",inline"`
		}{}
		iter := q.Iterator()
		defer iter.Close()
		workout := Workout{}
		count := 0
		for iter.Next(&row) {

			count++
		}
		if err = iter.Err(); err != nil {
			c.String(http.StatusInternalServerError, "Error reading workout. "+err.Error())
			return
		}
		if count == 0 {
			c.String(http.StatusBadRequest, "No workout matching that ID. "+err.Error())
			return
		}
		c.HTML(http.StatusOK, "workout.tmpl", workout)
	})

	router.GET("/deleteWorkout/:id", func(c *gin.Context) {
		workoutID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid workout ID.")
			return
		}
		userCookie, err := c.Cookie("user_id")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}
		user := UserDB{}
		err = db.Collection("users").Find(up.Cond{"cookie": userCookie}).One(&user)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading user info. Your user cookie may be invalid. "+err.Error())
			return
		}
		err = db.Collection("workouts").Find(workoutID).Delete()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error deleting workout session. "+err.Error())
			return
		}
		c.Redirect(http.StatusSeeOther, "/")
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

	router.GET("/admin/workouts", func(c *gin.Context) {
		var workouts []WorkoutDB
		err := db.Collection("workouts").Find().All(&workouts)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading workouts. "+err.Error())
			return
		}
		c.HTML(http.StatusOK, "admin_workouts.tmpl", workouts)
	})

	router.GET("/admin/set/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.String(http.StatusBadRequest, "Error bad set id. "+err.Error())
			return
		}
		var set SetDB
		err = db.Collection("sets").Find(id).One(&set)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading set. "+err.Error())
			return
		}
		c.HTML(http.StatusOK, "admin_set_edit.tmpl", set)
	})

	router.GET("/admin/workout/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.String(http.StatusBadRequest, "Error bad workout id. "+err.Error())
			return
		}
		var workout WorkoutDB
		err = db.Collection("workouts").Find(id).One(&workout)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading workouts. "+err.Error())
			return
		}
		var sets []SetDB
		err = db.Collection("sets").Find(up.Cond{"workout": id}).All(&sets)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading sets. "+err.Error())
			return
		}
		data := struct {
			WorkoutDB
			Sets []SetDB
		}{}
		data.WorkoutDB = workout
		data.Sets = sets
		c.HTML(http.StatusOK, "admin_workout_edit.tmpl", data)
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
		// todo: remove all workouts and sets associated with the user
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

	router.POST("/json/addWorkout", func(c *gin.Context) {
		var workout WorkoutDB
		c.MustBindWith(&workout, binding.JSON)
		_, err := db.Collection("workouts").Insert(workout)
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't add new workout. "+err.Error())
			return
		}
		c.String(http.StatusOK, workout.Name)
	})

	router.POST("/json/removeWorkout", func(c *gin.Context) {
		buf := &bytes.Buffer{}
		buf.ReadFrom(c.Request.Body)
		s := buf.String()

		// todo: also remove any sets associated with the workout
		workoutID, err := strconv.Atoi(s)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid id for workout to remove. "+err.Error())
			return
		}
		err = db.Collection("workouts").Find(workoutID).Delete()
		if err != nil {
			c.String(http.StatusInternalServerError, "Couldn't remove workouts. "+err.Error())
			return
		}
		c.String(http.StatusOK, "removed workout with id: "+s)
	})

	router.Run(":" + port)
}
