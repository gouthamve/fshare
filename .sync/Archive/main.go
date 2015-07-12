package main

import (
	"log"
	"net/http"

	"github.com/gouthamve/fshare/handlers"
	"github.com/julienschmidt/httprouter"
)

func main() {
	router := httprouter.New()
	router.GET("/file/:id", handlers.ServeFileHandler)
	router.POST("/file/:id", handlers.AddFileHandler)
	router.DELETE("/file/:id", handlers.RemoveFileHandler)

	router.GET("/members", handlers.GetMembers)

	router.GET("/members/:name/files", handlers.GetMemberFiles)
	router.GET("/files/", handlers.GetAllFiles)
	router.GET("/files/active", handlers.GetActiveFiles)

	log.Fatal(http.ListenAndServe("0.0.0.0:8080", router))
}
