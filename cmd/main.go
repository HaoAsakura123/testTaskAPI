package main

import (
	"context"
	"log"
	"net/http"
	"testTaskAPI/internal/app"
	conndb "testTaskAPI/internal/connDB"
	"testTaskAPI/internal/contextkeys"

	_ "testTaskAPI/docs"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title TestTaskAPI
// @version 1.0
// @description Это тестовый API s
// @host localhost:8080
// @BasePath /
func main() {

	connStr, DB_USER, err := conndb.PreparingBD()
	if err != nil {
		log.Fatal("DEBUG: Ошибка подключения к БД:", err)
	}

	db, err := sqlx.Connect(DB_USER, connStr)

	if err != nil {
		log.Fatal("DEBUG: Ошибка подключения к БД:", err)
	}
	defer db.Close()

	if err := conndb.RunMigrations(connStr); err != nil {
		log.Fatal("DEBUG: Ошибка миграций:", err)
	}

	router := http.NewServeMux()
	router.HandleFunc("/add", app.AddHandle)
	router.HandleFunc("/search", app.SearchHandle)
	router.HandleFunc("/delete/", app.DeleteHandle)
	router.HandleFunc("/update/", app.UpdateHandle)
	router.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	handler := dbMiddleware(db)(router)

	log.Printf("INFO: Server started at :8080\n")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal("DEBUG: Ошибка сервера:", err)
	}
}

func dbMiddleware(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			log.Println("INFO: Добавляем DB в контекст")
			ctx := context.WithValue(r.Context(), contextkeys.DB, db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
