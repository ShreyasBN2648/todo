package main

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/thedevsaddam/renderer"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

var rndr *renderer.Render
var db *mgo.Database

const (
	hostName       string = "localhost:27017"
	dbName         string = "demo_todo"
	collectionName string = "todo"
	port           string = ":9000"
)

type (
	todoModel struct {
		ID        bson.ObjectId `bson:"_id, omitempty"`
		Title     string        `bson:"title"`
		Completed bool          `bson:"completed"`
		CreatedAt time.Time     `bson:"createdAt"`
	}

	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"createdAt"`
	}
)

func init() {
	rndr = renderer.New()
	session, err := mgo.Dial(hostName)
	checkerr(err)
	session.SetMode(mgo.Monotonic, true)
	db = session.DB(dbName)
}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Mount("/todo", todoHandler())

	srv := &http.Server{
		Addr:         port,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on the port", port)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down the server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		checkerr(err)
	}
	defer cancel()
	log.Println("Server successfully shutdown.")
}

func checkerr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rndr.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	checkerr(err)

}

func todoHandler() http.Handler {
	rg := chi.NewRouter()
	rg.Group(func(r chi.Router) {
		r.Post("/", createTodo)
		r.Get("/", fetchTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})
	return rg
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		if err1 := rndr.JSON(w, http.StatusProcessing, err); err1 != nil {
			checkerr(err1)
		}
		return
	}

	if t.Title == "" {
		rndr.JSON(w, http.StatusProcessing, renderer.M{
			"error": "The title cannot be empty",
		})
		return
	}

	tm := todoModel{
		ID:        bson.NewObjectId(),
		Title:     t.Title,
		Completed: t.Completed,
		CreatedAt: t.CreatedAt,
	}

	if err := db.C(collectionName).Insert(&tm); err != nil {
		if err1 := rndr.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to create TODO",
			"error":   err,
		}); err1 != nil {
			checkerr(err1)
		}
		return
	}

	rndr.JSON(w, http.StatusCreated, renderer.M{
		"message": "TODO created successfully",
		"todo_id": tm.ID.Hex(),
	})
}

func fetchTodo(w http.ResponseWriter, r *http.Request) {
	todos := []todoModel{}

	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil {
		if err1 := rndr.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to fetch todo",
			"error":   err,
		}); err1 != nil {
			checkerr(err1)
		}
		return
	}

	todoList := []todo{}

	for _, t := range todos {
		todoList = append(todoList, todo{
			ID:        t.ID.Hex(),
			Title:     t.Title,
			Completed: t.Completed,
			CreatedAt: t.CreatedAt,
		})
	}
	if err1 := rndr.JSON(w, http.StatusOK, renderer.M{
		"data": todoList,
	}); err1 != nil {
		checkerr(err1)
		return
	}
}

func updateTodo(w http.ResponseWriter, r *http.Request) {

}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if bson.IsObjectIdHex(id) {
		if err1 := rndr.JSON(w, http.StatusBadRequest, renderer.M{
			"error": "Invalid URL request",
		}); err1 != nil {
			checkerr(err1)
			return
		}
		return
	}

	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(id)); err != nil {
		if err1 := rndr.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Failed to remove TODO",
			"error":   err,
		}); err1 != nil {
			checkerr(err1)
			return
		}
		return
	}

}
