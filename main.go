package main

import (
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/turamant/restserver/internal/taskstore"
	"github.com/turamant/restserver/middleware"

)

// type LoggingMiddleware struct {
// 	handler http.Handler
//   }
  
//   func (lm *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, req *http.Request) {
// 	start := time.Now()
// 	lm.handler.ServeHTTP(w, req)
// 	log.Printf("%s %s %s", req.Method, req.RequestURI, time.Since(start))
//   }

type Store struct {
	store *taskstore.TaskStore
}

func NewStore() *Store {
	return &Store{
		store: taskstore.New(),
	}
}

func (s *Store) task(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/task/" {
		// Request is plain "/task/", without trailing ID.
		if req.Method == http.MethodPost {
			s.createTaskHandler(w, req)
		} else if req.Method == http.MethodGet {
			s.getAllTasksHandler(w, req)
		} else if req.Method == http.MethodDelete {
			s.deleteAllTasksHandler(w, req)
		} else {
			http.Error(w, fmt.Sprintf("expect method GET, DELETE or POST at /task/, got %v", req.Method), http.StatusMethodNotAllowed)
			return
		}
	} else {
		// Request has an ID, as in "/task/<id>".
		path := strings.Trim(req.URL.Path, "/")
		pathParts := strings.Split(path, "/")
		if len(pathParts) < 2 {
			http.Error(w, "expect /task/<id> in task handler", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(pathParts[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Method == http.MethodDelete {
			s.deleteTaskHandler(w, req, id)
		} else if req.Method == http.MethodGet {
			s.getTaskHandler(w, req, id)
		} else {
			http.Error(w, fmt.Sprintf("expect method GET or DELETE at /task/<id>, got %v", req.Method), http.StatusMethodNotAllowed)
			return
		}
	}
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

func (s *Store) getTaskHandler(w http.ResponseWriter, req *http.Request, id int) {
	log.Printf("handling get task at %s\n", req.URL.Path)
	
	task, err := s.store.GetTask(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	renderJSON(w, task)
}

func (s *Store) deleteTaskHandler(w http.ResponseWriter, req *http.Request, id int) {
	log.Printf("handling delete task at %s\n", req.URL.Path)
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

	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("expect method GET /tag/<tag>, got %v", req.Method), http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(req.URL.Path, "/")
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 {
		http.Error(w, "expect /tag/<tag> path", http.StatusBadRequest)
		return
	}
	tag := pathParts[1]

	tasks := s.store.GetTasksByTag(tag)
	renderJSON(w, tasks)
}

func (s *Store) due(w http.ResponseWriter, req *http.Request) {
	log.Printf("handling tasks by due at %s\n", req.URL.Path)

	if req.Method != http.MethodGet {
		http.Error(w, fmt.Sprintf("expect method GET /due/<date>, got %v", req.Method), http.StatusMethodNotAllowed)
		return
	}

	path := strings.Trim(req.URL.Path, "/")
	pathParts := strings.Split(path, "/")

	badRequestError := func() {
		http.Error(w, fmt.Sprintf("expect /due/<year>/<month>/<day>, got %v", req.URL.Path), http.StatusBadRequest)
	}
	if len(pathParts) != 4 {
		badRequestError()
		return
	}

	year, err := strconv.Atoi(pathParts[1])
	if err != nil {
		badRequestError()
		return
	}
	month, err := strconv.Atoi(pathParts[2])
	if err != nil || month < int(time.January) || month > int(time.December) {
		badRequestError()
		return
	}
	day, err := strconv.Atoi(pathParts[3])
	if err != nil {
		badRequestError()
		return
	}

	tasks := s.store.GetTasksByDueDate(year, time.Month(month), day)
	renderJSON(w, tasks)
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

func main() {
	store := NewStore()
	router := http.NewServeMux()
	lm := &middleware.LoggingMiddleware{Handler: router}
	server := http.Server{
		Addr:           "localhost:" + os.Getenv("SERVERPORT"),
		Handler:        lm,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	router.HandleFunc("/task/", store.task)
	router.HandleFunc("/tag/", store.tag)
	router.HandleFunc("/due/", store.due)
	log.Fatal(server.ListenAndServe())
}
