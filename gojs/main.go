package main

import (
	"github.com/gopherjs/gopherjs/js"
)

type User struct {
	Name string
	// todo how is int64 represented in JS?
	Workouts  map[int64]*Workout // indexed by StartTime
	Templates []Workout          // user's favorite templates
	Exercises []Exercise         // user's favorite exercises (SetsExpected used but Sets is empty)
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
	Rest     int // time in milliseconds of rest before next set/exercise
}

func main() {
	doc := js.Global.Get("document")
	println("Hello, browser console!", doc)
}
