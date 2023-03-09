package main

import (
	"net/http"
)

func main() {
	//keeping alive on replit
	http.HandleFunc("/", handle)
	go http.ListenAndServe(":0", nil)

	storage, err := CreateMongoStorage()
	if err != nil {
		panic(err)
	}

	worker := CreateDouWorker(storage)
	if err := worker.Run(); err != nil {
		panic(err)
	}

	bot := CreateTelegramBot(storage, worker)
	bot.Run()
}

func handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/text")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
	return
}
