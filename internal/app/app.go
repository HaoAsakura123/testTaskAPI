package app

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testTaskAPI/internal/contextkeys"
	"testTaskAPI/internal/getAPI"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	_ "testTaskAPI/docs"
	_ "github.com/lib/pq"
)
// Info represents person information
// @Description Person information with details from external APIs
type Info struct {
	HumanID     string    `json:"id" db:"human_id"`
	Name        string    `json:"name" db:"name"`
	Surname     string    `json:"surname" db:"surname"`
	Patronymic  string    `json:"patronymic" db:"patronymic"`
	Age         int       `json:"age" db:"age"`
	Genderize   string    `json:"gender" db:"gender"`
	Nationalize string    `json:"country" db:"country"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// @Summary Add user
// @Description Adds a new user to the database
// @Tags users
// @Accept json
// @Produce json
// @Param user body model.User true "User data"
// @Success 200 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Router /add [post]

func AddHandle(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		log.Println("DEBUG: Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	// 1. Получаем DB из контекста с правильным ключом
	dbVal := r.Context().Value(contextkeys.DB)
	if dbVal == nil {
		log.Println("DEBUG: DB connection is NIL in context")
		http.Error(w, "Database connection not established", http.StatusInternalServerError)
		return
	}

	db, ok := dbVal.(*sqlx.DB)
	if !ok {
		log.Printf("DEBUG: Expected *sqlx.DB, got %T", dbVal)
		http.Error(w, "Invalid database connection type", http.StatusInternalServerError)
		return
	}

	// 2. Парсим запрос
	var infoAbout Info
	if err := json.NewDecoder(r.Body).Decode(&infoAbout); err != nil {
		log.Printf("DEBUG: JSON decode error: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. Генерируем ID
	infoAbout.HumanID = uuid.New().String()

	// 4. Получаем данные из API
	tmpPerson, err := getAPI.GetAPI(infoAbout.Name)
	if err != nil {
		log.Printf("DEBUG: API error: %v", err)
		http.Error(w, "Failed to get additional data", http.StatusBadGateway)
		return
	}

	// 5. Заполняем данные
	infoAbout.Age = tmpPerson.Age
	infoAbout.Genderize = tmpPerson.Gender
	infoAbout.Nationalize = tmpPerson.Country

	// 6. Сохраняем в БД
	if err := saveToDB(db, infoAbout); err != nil {
		log.Printf("DEBUG: DB save error: %v", err)
		http.Error(w, "Failed to save data", http.StatusInternalServerError)
		return
	}

	// 7. Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"humanid": infoAbout.HumanID,
		"data":    infoAbout,
	}); err != nil {
		log.Printf("DEBUG: JSON encode error: %v", err)
	}
}

func saveToDB(db *sqlx.DB, info Info) error {
	query := `
		INSERT INTO people 
		(human_id, name, surname, patronymic, age, gender, country) 
		VALUES 
		(:human_id, :name, :surname, :patronymic, :age, :gender, :country)`
	_, err := db.NamedExec(query, info)
	return err
}

// SearchHandle ищет пользователей
// @Summary Поиск пользователей
// @Description Получение списка пользователей по заданным фильтрам
// @Tags users
// @Accept json
// @Produce json
// @Param name query string false "Имя пользователя для поиска"
// @Param age query int false "Возраст пользователя для поиска"
// @Param country query string false "Страна пользователя для поиска"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Количество записей на страницу"
// @Param sort_by query string false "Поле для сортировки (например, created_at)"
// @Param sort_order query string false "Порядок сортировки (ASC или DESC)"
// @Success 200 {object} map[string]interface{} "Результат поиска с пагинацией"
// @Failure 500 {string} string "Внутренняя ошибка сервера"
// @Router /search [get]

func SearchHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем подключение к БД
	db, ok := r.Context().Value(contextkeys.DB).(*sqlx.DB)
	if !ok {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// Парсим параметры запроса
	queryParams := r.URL.Query()

	// Пагинация
	page, limit := getPaginationParams(queryParams)
	offset := (page - 1) * limit

	// Фильтры
	filters := getFilters(queryParams)

	// Формируем SQL запрос
	baseQuery := "SELECT human_id, name, surname, patronymic, age, gender, country, created_at FROM people WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM people WHERE 1=1"

	var args []interface{}
	var countArgs []interface{}
	argCounter := 1

	// Добавляем фильтры
	for key, value := range filters {
		switch key {
		case "name":
			baseQuery += fmt.Sprintf(" AND name ILIKE $%d", argCounter)
			countQuery += fmt.Sprintf(" AND name ILIKE $%d", argCounter)
			args = append(args, "%"+value+"%")
			countArgs = append(countArgs, "%"+value+"%")
			argCounter++
		case "age":
			baseQuery += fmt.Sprintf(" AND age = $%d", argCounter)
			countQuery += fmt.Sprintf(" AND age = $%d", argCounter)
			args = append(args, value)
			countArgs = append(countArgs, value)
			argCounter++
		case "country":
			baseQuery += fmt.Sprintf(" AND country = $%d", argCounter)
			countQuery += fmt.Sprintf(" AND country = $%d", argCounter)
			args = append(args, value)
			countArgs = append(countArgs, value)
			argCounter++
		}
	}

	// Сортировка
	sortBy := queryParams.Get("sort_by")
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := queryParams.Get("sort_order")
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)

	// Пагинация
	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
	args = append(args, limit, offset)

	// Выполняем запрос
	var people []Info
	err := db.Select(&people, baseQuery, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Получаем общее количество
	var total int
	err = db.Get(&total, countQuery, countArgs...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	response := map[string]interface{}{
		"data": people,
		"pagination": map[string]interface{}{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Вспомогательные функции
func getPaginationParams(queryParams url.Values) (page, limit int) {
	page = 1
	if p := queryParams.Get("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	limit = 10
	if l := queryParams.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
			if limit > 100 {
				limit = 100
			}
		}
	}
	return
}

func getFilters(queryParams url.Values) map[string]string {
	filters := make(map[string]string)
	if name := queryParams.Get("name"); name != "" {
		filters["name"] = name
	}
	if age := queryParams.Get("age"); age != "" {
		filters["age"] = age
	}
	if country := queryParams.Get("country"); country != "" {
		filters["country"] = country
	}
	return filters
}

// DeleteHandle удаляет пользователя
// @Summary Удалить пользователя
// @Description Удаляет пользователя по его ID
// @Tags users
// @Param id path string true "ID пользователя для удаления"
// @Success 204 "Пользователь успешно удалён"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 404 {string} string "Пользователь не найден"
// @Failure 500 {string} string "Внутренняя ошибка сервера"
// @Router /delete/{id} [delete]

func DeleteHandle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idToDelete := r.URL.Path[len("/delete/"):]

	if idToDelete == "" {
		log.Print("DEBUG: empty id\n")
		http.Error(w, "empty id", http.StatusBadRequest)
		return
	}

	// Получаем подключение к БД
	db, ok := r.Context().Value(contextkeys.DB).(*sqlx.DB)
	if !ok {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// 2. Выполняем SQL-запрос на удаление
	query := `DELETE FROM people WHERE human_id = $1`
	result, err := db.Exec(query, idToDelete)
	if err != nil {
		log.Printf("DEBUG: delete failed: %v\n", err)
		http.Error(w, "failed to delete person", http.StatusInternalServerError)
		return
	}

	// 3. Проверяем, была ли удалена хотя бы одна запись
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("DEBUG: failed to check affected rows: %v\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "person not found", http.StatusNotFound)
		return
	}
	log.Println("INFO: Success delete person")
	// 4. Возвращаем успешный статус (204 No Content)
	w.WriteHeader(http.StatusNoContent)

}

// UpdateHandle обновляет данные пользователя
// @Summary Обновить пользователя
// @Description Частичное обновление данных пользователя по ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "ID пользователя для обновления"
// @Param input body map[string]interface{} true "Данные для обновления"
// @Success 200 {object} map[string]interface{} "Успешный ответ с обновлёнными данными"
// @Failure 400 {string} string "Ошибка в запросе"
// @Failure 404 {string} string "Пользователь не найден"
// @Failure 500 {string} string "Внутренняя ошибка сервера"
// @Router /update/{id} [patch]

func UpdateHandle(w http.ResponseWriter, r *http.Request) {
	// 1. Проверяем метод PATCH
	if r.Method != http.MethodPatch {
		log.Println("DEBUG: Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. Извлекаем DB из контекста (аналогично AddHandle)
	dbVal := r.Context().Value(contextkeys.DB)
	if dbVal == nil {
		log.Println("DEBUG: DB connection is NIL in context")
		http.Error(w, "Database connection not established", http.StatusInternalServerError)
		return
	}

	db, ok := dbVal.(*sqlx.DB)
	if !ok {
		log.Printf("DEBUG: Expected *sqlx.DB, got %T", dbVal)
		http.Error(w, "Invalid database connection type", http.StatusInternalServerError)
		return
	}

	// 3. Получаем HumanID из URL (например, /humans/123)

	// Или так для стандартного http:
	humanID := r.URL.Path[len("/delete/"):]
	if humanID == "" {
		http.Error(w, "HumanID is required", http.StatusBadRequest)
		return
	}

	// 4. Парсим ТОЛЬКО переданные поля (частичное обновление)
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		log.Printf("DEBUG: JSON decode error: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 5. Динамически генерируем SQL-запрос
	query := "UPDATE people SET "
	var args []interface{}
	i := 1

	// Проверяем допустимые поля для обновления
	allowedFields := map[string]bool{
		"name":       true,
		"surname":    true,
		"patronymic": true,
		"age":        true,
		"gender":     true,
		"country":    true,
	}

	for key, val := range updates {
		if !allowedFields[key] {
			log.Printf("DEBUG: Invalid field: %s", key)
			http.Error(w, fmt.Sprintf("Field '%s' cannot be updated", key), http.StatusBadRequest)
			return
		}
		query += fmt.Sprintf("%s = $%d, ", key, i)
		args = append(args, val)
		i++
	}

	if len(args) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	// Удаляем последнюю запятую и добавляем условие WHERE
	query = strings.TrimSuffix(query, ", ") + " WHERE human_id = $" + strconv.Itoa(i)
	args = append(args, humanID)

	// 6. Выполняем запрос
	_, err := db.Exec(query, args...)
	if err != nil {
		log.Printf("DEBUG: DB update error: %v", err)
		http.Error(w, "Failed to update data", http.StatusInternalServerError)
		return
	}

	// 7. Возвращаем обновленные данные (опционально)
	updatedInfo := Info{}
	err = db.Get(&updatedInfo, "SELECT * FROM people WHERE human_id = $1", humanID)
	if err != nil {
		log.Printf("DEBUG: DB fetch error: %v", err)
		http.Error(w, "Failed to fetch updated data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   updatedInfo,
	}); err != nil {
		log.Printf("DEBUG: JSON encode error: %v", err)
	}
}
