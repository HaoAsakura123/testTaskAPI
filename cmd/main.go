package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testTaskAPI/internal/app"
	"testTaskAPI/internal/connDB"
	"testTaskAPI/internal/contextkeys"

	"github.com/joho/godotenv"
	"github.com/swaggo/http-swagger" 
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "testTaskAPI/docs"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("DEBUG: No .env file found, using system environment variables")
	}
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	DB_USER := os.Getenv("DB_USER")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_NAME := os.Getenv("DB_NAME")
	DB_SSLMODE := os.Getenv("DB_SSLMODE")
	SERVER_ADDR := os.Getenv("SERVER_ADDR")
	// 1. Создание БД
	err := conndb.CreateDatabaseIfNotExists(DB_USER, DB_PASSWORD, DB_NAME)
	if err != nil {
		log.Fatal("DEBUG: Ошибка создания БД:", err)
	}

	// 2. Подключение к БД
	connStr := DB_USER + "://" + DB_USER + ":" + DB_PASSWORD + "@" + dbHost + ":" + dbPort + "/" + DB_NAME + "?sslmode=" + DB_SSLMODE
	db, err := sqlx.Connect(DB_USER, connStr)
	if err != nil {
		log.Fatal("DEBUG: Ошибка подключения к БД:", err)
	}
	defer db.Close()

	// 3. Проверка подключения
	if err = db.Ping(); err != nil {
		log.Fatal("DEBUG: Ошибка ping БД:", err)
	}

	// 4. Применение миграций
	if err := runMigrations(connStr); err != nil {
		log.Fatal("DEBUG: Ошибка миграций:", err)
	}

	// 5. Настройка маршрутизатора
	router := http.NewServeMux()
	router.HandleFunc("/add", app.AddHandle)
	router.HandleFunc("/search", app.SearchHandle)
	router.HandleFunc("/delete/", app.DeleteHandle)
	router.HandleFunc("/update/", app.UpdateHandle)
	router.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	// 6. Обертываем роутер в middleware
	handler := dbMiddleware(db)(router)

	// 7. Запуск сервера
	log.Printf("INFO: Server started at %s\n", SERVER_ADDR)
	if err := http.ListenAndServe(SERVER_ADDR, handler); err != nil {
		log.Fatal("DEBUG: Ошибка сервера:", err)
	}
}

func dbMiddleware(db *sqlx.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Добавляем лог для отладки
			log.Println("INFO: Добавляем DB в контекст")
			ctx := context.WithValue(r.Context(), contextkeys.DB, db)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func runMigrations(connStr string) error {
	m, err := migrate.New(
		"file://../migrations", // Измененный путь
		connStr,
	)
	if err != nil {
		return fmt.Errorf("ошибка инициализации миграций: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}

	log.Println("INFO: Миграции успешно применены")
	return nil
}
