package conndb

import (
	"database/sql"
	"fmt"
	_ "github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"log"
	"os"
)

func CreateDatabaseIfNotExists(dbUser, dbPassword, dbName string) error {
	// Загрузка .env (путь должен быть правильным)
	if err := godotenv.Load("cmd/.env"); err != nil {
		log.Println("DEBUG: No .env file found, using system environment variables")
	}
	DB_SSLMODE := os.Getenv("DB_SSLMODE")

	// Подключаемся к системной БД (например, postgres), чтобы проверить/создать целевую БД
	systemConnStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s",
		dbUser, dbPassword, dbUser, DB_SSLMODE)

	// Используем драйвер "postgres", а не имя вашей БД!
	db, err := sql.Open("postgres", systemConnStr)
	if err != nil {
		log.Println("DEBUG: failed to connect to system database")
		return fmt.Errorf("failed to connect to system database: %v", err)
	}
	defer db.Close()

	// Проверяем существование БД
	var exists bool
	err = db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM pg_database WHERE datname = $1
        )`, dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %v", err)
	}

	// Создаём БД, если её нет
	if !exists {
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database: %v", err)
		}
		log.Printf("INFO: Database %s created successfully", dbName)
	}

	return nil
}
