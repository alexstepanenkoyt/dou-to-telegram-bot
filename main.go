package main

import (
	"encoding/json"
	"net/http"
)

func main() {
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

	http.HandleFunc("/", handle)
	http.ListenAndServe(":0", nil)
}

func handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/text")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Success"))
	return
}
func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}
