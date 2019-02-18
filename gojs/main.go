package main

/*
JSON marshal/unmarshal functions from https://github.com/johanbrandhorst/gopherjs-json MIT license

*/

import (
	"github.com/gopherjs/gopherjs/js"
	"honnef.co/go/js/dom"
	"honnef.co/go/js/xhr"
)

type User struct {
	Name     string
	Password string
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

type UserDB struct {
	ID       uint64 `db:"id,omitempty"`
	Name     string `db:"name"`
	Password string `db:"password"`
}

// Marshal uses the browser builtin JSON.stringify function
// and wraps it such that any exceptions thrown are returned
// as errors.
func MarshalObj(o *js.Object) (res string, err error) {
	defer func() {
		e := recover()

		if e == nil {
			return
		}

		if e, ok := e.(*js.Error); ok {
			err = e
		} else {
			panic(e)
		}
	}()

	res = js.Global.Get("JSON").Call("stringify", o).String()

	return res, err
}

func Marshal(o interface{}) (res string, err error) {
	defer func() {
		e := recover()

		if e == nil {
			return
		}

		if e, ok := e.(*js.Error); ok {
			err = e
		} else {
			panic(e)
		}
	}()

	res = js.Global.Get("JSON").Call("stringify", o).String()

	return res, err
}

// Unmarshal uses the browser builtin JSON.parse function
// and wraps it such that any exceptions thrown are returned
// as errors.
func Unmarshal(s string) (res *js.Object, err error) {
	defer func() {
		e := recover()

		if e == nil {
			return
		}

		if e, ok := e.(*js.Error); ok {
			err = e
		} else {
			panic(e)
		}
	}()

	res = js.Global.Get("JSON").Call("parse", s)

	return res, err
}

func reload() {
	js.Global.Get("location").Call("reload")
}

func pageAdminUsers() {
	button := doc.GetElementByID("add_button").(*dom.HTMLButtonElement)
	userNameText := doc.GetElementByID("user_name_text").(*dom.HTMLInputElement)
	userList := doc.GetElementByID("user_list")

	button.AddEventListener("click", false, func(evt dom.Event) {
		go func() {
			req := xhr.NewRequest("POST", "/json/addUser")
			req.Timeout = 1000 // milliseconds
			req.ResponseType = xhr.Text
			err := req.Send(userNameText.Value)
			if err != nil {
				println(err)
				return
			}
			println("success adding new user: ", req.ResponseText)
			reload()
		}()
	})

	userList.AddEventListener("click", false, func(evt dom.Event) {
		userID := evt.Target().GetAttribute("userID")
		evt.PreventDefault()
		go func() {
			req := xhr.NewRequest("POST", "/json/removeUser")
			req.Timeout = 1000 // milliseconds
			req.ResponseType = xhr.Text
			err := req.Send(userID)
			if err != nil {
				println(err)
				return
			}
			println("success removing user: ", req.ResponseText)
			reload()
		}()
	})
}

func pageAdminExercises() {
	button := doc.GetElementByID("add_button").(*dom.HTMLButtonElement)
	exerciseNameText := doc.GetElementByID("exercise_name_text").(*dom.HTMLInputElement)
	exerciseNotesText := doc.GetElementByID("exercise_notes_text").(*dom.HTMLTextAreaElement)
	exerciseList := doc.GetElementByID("exercise_list")

	button.AddEventListener("click", false, func(evt dom.Event) {
		go func() {
			req := xhr.NewRequest("POST", "/json/addExercise")
			req.Timeout = 1000 // milliseconds
			req.ResponseType = xhr.Text
			req.SetRequestHeader("Content-Type", "application/json")
			json, err := Marshal(
				map[string]string{
					"name":  exerciseNameText.Value,
					"notes": exerciseNotesText.Value,
				},
			)
			if err != nil {
				println(err)
				return
			}
			err = req.Send(json)
			if err != nil {
				println(err)
				return
			}
			println("success adding new user: ", req.ResponseText)
			reload()
		}()
	})

	exerciseList.AddEventListener("click", false, func(evt dom.Event) {
		exerciseID := evt.Target().GetAttribute("exerciseID")
		evt.PreventDefault()
		go func() {
			req := xhr.NewRequest("POST", "/json/removeExercise")
			req.Timeout = 1000 // milliseconds
			req.ResponseType = xhr.Text
			err := req.Send(exerciseID)
			if err != nil {
				println(err)
				return
			}
			println("success removing exercise: ", req.ResponseText)
			reload()
		}()
	})
}

var doc dom.Document

func main() {
	doc = dom.GetWindow().Document()
	js.Global.Set("pageAdminUsers", pageAdminUsers)
	js.Global.Set("pageAdminExercises", pageAdminExercises)
}
