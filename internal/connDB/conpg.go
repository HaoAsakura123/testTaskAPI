package conndb

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func CreateDatabaseIfNotExists(dbUser, dbPassword, dbName string) error {

	if err := godotenv.Load(".env"); err != nil {
		log.Println("DEBUG: No .env file found, using system environment variables")
	}
	DB_SSLMODE := os.Getenv("DB_SSLMODE")

	systemConnStr := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=%s",
		dbUser, dbPassword, dbUser, DB_SSLMODE)

	db, err := sql.Open("postgres", systemConnStr)
	if err != nil {
		log.Println("DEBUG: failed to connect to system database")
		return fmt.Errorf("failed to connect to system database: %v", err)
	}
	defer db.Close()

	var exists bool
	err = db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 FROM pg_database WHERE datname = $1
        )`, dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %v", err)
	}

	if !exists {
		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
		if err != nil {
			return fmt.Errorf("failed to create database: %v", err)
		}
		log.Printf("INFO: Database %s created successfully", dbName)
	}

	return nil
}


func RunMigrations(connStr string) error {
	m, err := migrate.New(
		"file://migrations",
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

func PreparingBD() (string, string, error){
	if err := godotenv.Load(".env"); err != nil {
		log.Println("DEBUG: No .env file found, using system environment variables")
		return "", "",err
	}
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_NAME := os.Getenv("DB_NAME")
	DB_SSLMODE := os.Getenv("DB_SSLMODE")
	DB_USER := os.Getenv("DB_USER")

	err := CreateDatabaseIfNotExists(DB_USER, DB_PASSWORD, DB_NAME)

	if err != nil {
		log.Fatal("DEBUG: Ошибка создания БД:", err)
		return "", "", err
	}
	connStr := DB_USER + "://" + DB_USER + ":" + DB_PASSWORD + "@" + dbHost + ":" + dbPort + "/" + DB_NAME + "?sslmode=" + DB_SSLMODE
	return connStr, DB_USER, nil
}