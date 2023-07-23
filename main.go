package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/turamant/restserver/internal/taskstore"
	"github.com/turamant/restserver/middleware"

	"github.com/gorilla/mux"

)

type Store struct {
	store *taskstore.TaskStore
}

func NewStore() *Store {
	return &Store{
		store: taskstore.New(),
	}
}

// renderJSON renders 'v' as JSON and writes it as a response into w.
func renderJSON(w http.ResponseWriter, v interface{}) {
	js, err := json.Marshal(v)
	if err != nil {
	  http.Error(w, err.Error(), http.StatusInternalServerError)
	  return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func (s *Store) createTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling task create at %s\n", req.URL.Path)

	// Types used internally in this handler to (de-)serialize the request and
	// response from/to JSON.
	type RequestTask struct {
		Text string    `json:"text"`
		Tags []string  `json:"tags"`
		Due  time.Time `json:"due"`
	}

	type ResponseId struct {
		Id int `json:"id"`
	}

	// Enforce a JSON Content-Type.
	contentType := req.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if mediatype != "application/json" {
		http.Error(w, "expect application/json Content-Type", http.StatusUnsupportedMediaType)
		return
	}

	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()
	var rt RequestTask
	if err := dec.Decode(&rt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := s.store.CreateTask(rt.Text, rt.Tags, rt.Due)
	renderJSON(w, id)
}

func (s *Store) getAllTasksHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling get all tasks at %s\n", req.URL.Path)
	allTasks := s.store.GetAllTasks()
	renderJSON(w, allTasks)
}

func (s *Store) getTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling get task at %s\n", req.URL.Path)
	
	id, _ := strconv.Atoi(mux.Vars(req)["id"])
	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	renderJSON(w, task)
}

func (s *Store) deleteTaskHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling delete task at %s\n", req.URL.Path)
	id, _ := strconv.Atoi(mux.Vars(req)["id"])
	err := s.store.DeleteTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}
}

func (s *Store) deleteAllTasksHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling delete all tasks at %s\n", req.URL.Path)
	s.store.DeleteAllTasks()
}

func (s *Store) tag(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling tasks by tag at %s\n", req.URL.Path)
	
	tag := mux.Vars(req)["tag"]
	tasks := s.store.GetTasksByTag(tag)
	renderJSON(w, tasks)
}

func (s *Store) due(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling tasks by due at %s\n", req.URL.Path)

	vars := mux.Vars(req)
	badRequestError := func() {
		http.Error(w, fmt.Sprintf("expect /due/<year>/<month>/<day>, got %v", req.URL.Path), http.StatusBadRequest)
	}

	year, _ := strconv.Atoi(vars["year"])
	month, _ := strconv.Atoi(vars["month"])

	if month < int(time.January) || month > int(time.December) {
		badRequestError()
		return
	}
	day, _ := strconv.Atoi(vars["day"])

	tasks := s.store.GetTasksByDueDate(year, time.Month(month), day)
	renderJSON(w, tasks)
}


func main() {
	store := NewStore()
	router := mux.NewRouter()
	router.StrictSlash(true)
	lm := &middleware.LoggingMiddleware{Handler: router}
	server := http.Server{
		Addr:           "localhost:" + os.Getenv("SERVERPORT"),
		Handler:        lm,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	router.HandleFunc("/task/", store.createTaskHandler).Methods("POST")
	router.HandleFunc("/task/", store.getAllTasksHandler).Methods("GET")
	router.HandleFunc("/task/", store.deleteAllTasksHandler).Methods("DELETE")
	router.HandleFunc("/task/{id:[0-9]+}/", store.getTaskHandler).Methods("GET")
	router.HandleFunc("/task/{id:[0-9]+}/", store.deleteTaskHandler).Methods("DELETE")
	router.HandleFunc("/tag/{tag}/", store.tag).Methods("GET")
	router.HandleFunc("/due/{year:[0-9]+}/{month:[0-9]+}/{day:[0-9]+}/", store.due).Methods("GET")

	log.Fatal(server.ListenAndServe())
}
