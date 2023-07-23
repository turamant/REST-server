package middleware


import (
	"log"
	"net/http"
    "time"
)
type LoggingMiddleware struct {
	Handler http.Handler
  }
  
  func (lm *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	lm.Handler.ServeHTTP(w, req)
	log.Printf("%s %s %s", req.Method, req.RequestURI, time.Since(start))
  }