package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testTaskAPI/internal/contextkeys"
	"testTaskAPI/internal/getapi"
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
	Name        string    `json:"name" db:"name" binding:"required"`
	Surname     string    `json:"surname" db:"surname" binding:"required"`
	Patronymic  string    `json:"patronymic" db:"patronymic"`
	Age         int       `json:"age" db:"age"`
	Genderize   string    `json:"gender" db:"gender"`
	Nationalize string    `json:"country" db:"country"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type requestData struct{
	Name        string    `json:"name" db:"name" binding:"required"`
	Surname     string    `json:"surname" db:"surname" binding:"required"`
	Patronymic  string    `json:"patronymic" db:"patronymic"`
}

// @Summary Add user
// @Description Adds a new user to the database
// @Tags users
// @Accept json
// @Produce json
// @Param user body requestData true "User data"
// @Success 200 {object} Info
// @Failure 400 {object} string "error"
// @Router /add [post]
func AddHandle(w http.ResponseWriter, r *http.Request) {

    if !validateMethod(w, r, http.MethodPost) {
        return
    }
	
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
	
	var requestData requestData
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Printf("DEBUG: JSON decode error: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestData.Name == "" || requestData.Surname == "" {
		log.Printf("DEBUG: JSON decode error: Name and surname are required")
    	http.Error(w, "Name and surname are required", http.StatusBadRequest)
    	return
	}

	infoAbout, err := loadData(requestData)

	if err != nil {
		log.Printf("DEBUG: API error: %v", err)
		http.Error(w, "Failed to get additional data", http.StatusBadGateway)
		return 
	}

	if err := saveToDB(db, infoAbout); err != nil {
		log.Printf("DEBUG: DB save error: %v", err)
		http.Error(w, "Failed to save data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"humanid": infoAbout.HumanID,
		"data":    infoAbout,
	}); err != nil {
		log.Printf("DEBUG: JSON encode error: %v", err)
		http.Error(w, "DEBUG: JSON encode error", http.StatusInternalServerError)
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

func loadData(requestData requestData) (Info, error){
	info := Info{}
	tmpPerson, err := getapi.GetAPI(requestData.Name)
	if err != nil{
		log.Printf("INFO: cannot load info about %s %s", requestData.Name, requestData.Surname)
		return info, err
	}
	info.Name = requestData.Name
	info.Surname = requestData.Surname
	info.Patronymic = requestData.Patronymic
	info.HumanID = uuid.New().String()
	info.Age = tmpPerson.Age
	info.Genderize = tmpPerson.Gender
	info.Nationalize = tmpPerson.Country

	return info, nil
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
    if !validateMethod(w, r, http.MethodGet) {
        return
    }

	db, ok := r.Context().Value(contextkeys.DB).(*sqlx.DB)
	if !ok {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	queryParams := r.URL.Query()

	page, limit := getPaginationParams(queryParams)
	offset := (page - 1) * limit

	filters := getFilters(queryParams)

	baseQuery := "SELECT human_id, name, surname, patronymic, age, gender, country, created_at FROM people WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM people WHERE 1=1"

	var args []interface{}
	var countArgs []interface{}
	argCounter := 1

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

	sortBy := queryParams.Get("sort_by")
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := queryParams.Get("sort_order")
	if sortOrder == "" {
		sortOrder = "DESC"
	}
	baseQuery += fmt.Sprintf(" ORDER BY %s %s", sortBy, sortOrder)


	baseQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCounter, argCounter+1)
	args = append(args, limit, offset)

	var people []Info
	err := db.Select(&people, baseQuery, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var total int
	err = db.Get(&total, countQuery, countArgs...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
    if !validateMethod(w, r, http.MethodDelete) {
        return
    }

	idToDelete := r.URL.Path[len("/delete/"):]

	if idToDelete == "" {
		log.Print("DEBUG: empty id\n")
		http.Error(w, "empty id", http.StatusBadRequest)
		return
	}

	db, ok := r.Context().Value(contextkeys.DB).(*sqlx.DB)
	if !ok {
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	query := `DELETE FROM people WHERE human_id = $1`
	result, err := db.Exec(query, idToDelete)
	if err != nil {
		log.Printf("DEBUG: delete failed: %v\n", err)
		http.Error(w, "failed to delete person", http.StatusInternalServerError)
		return
	}

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
	w.WriteHeader(http.StatusOK)

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
    if !validateMethod(w, r, http.MethodPatch) {
        return
    }

    db, err := getDBFromContext(r.Context())
    if err != nil {
        respondWithError(w, "Database error", http.StatusInternalServerError, err)
        return
    }

    humanID := extractHumanID(r.URL.Path)
    if humanID == "" {
        respondWithError(w, "HumanID is required", http.StatusBadRequest, nil)
        return
    }

    updates, err := decodeAndValidateUpdates(r.Body)
    if err != nil {
        respondWithError(w, err.Error(), http.StatusBadRequest, err)
        return
    }

    if err := updatePersonInDB(db, humanID, updates); err != nil {
        respondWithError(w, "Update failed", http.StatusInternalServerError, err)
        return
    }

    updatedInfo, err := FetchUpdatedPerson(db, humanID)
    if err != nil {
        respondWithError(w, "Failed to fetch updated data", http.StatusInternalServerError, err)
        return
    }

    respondWithJSON(w, http.StatusOK, map[string]interface{}{
        "status": "success",
        "data":   updatedInfo,
    })
}

func validateMethod(w http.ResponseWriter, r *http.Request, expectedMethod string) bool {
    if r.Method != expectedMethod {
        log.Printf("DEBUG: Method not allowed. Expected %s, got %s", expectedMethod, r.Method)
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return false
    }
    return true
}

func getDBFromContext(ctx context.Context) (*sqlx.DB, error) {
    dbVal := ctx.Value(contextkeys.DB)
    if dbVal == nil {
        return nil, errors.New("DB connection is NIL in context")
    }
    
    db, ok := dbVal.(*sqlx.DB)
    if !ok {
        return nil, fmt.Errorf("expected *sqlx.DB, got %T", dbVal)
    }
    return db, nil
}

func extractHumanID(path string) string {
    const prefix = "/delete/"
    if len(path) <= len(prefix) {
        return ""
    }
    return path[len(prefix):]
}

func decodeAndValidateUpdates(body io.Reader) (map[string]interface{}, error) {
    var updates map[string]interface{}
    if err := json.NewDecoder(body).Decode(&updates); err != nil {
        return nil, fmt.Errorf("invalid request body: %v", err)
    }

    allowedFields := map[string]bool{
        "name":       true,
        "surname":    true,
        "patronymic": true,
        "age":        true,
        "gender":     true,
        "country":    true,
    }

    for key := range updates {
        if !allowedFields[key] {
            return nil, fmt.Errorf("field '%s' cannot be updated", key)
        }
    }

    if len(updates) == 0 {
        return nil, errors.New("no fields to update")
    }

    return updates, nil
}

func updatePersonInDB(db *sqlx.DB, humanID string, updates map[string]interface{}) error {
    query := "UPDATE people SET "
    var args []interface{}
    i := 1

    for key, val := range updates {
        query += fmt.Sprintf("%s = $%d, ", key, i)
        args = append(args, val)
        i++
    }

    query = strings.TrimSuffix(query, ", ") + " WHERE human_id = $" + strconv.Itoa(i)
    args = append(args, humanID)

    _, err := db.Exec(query, args...)
    return err
}

func FetchUpdatedPerson(db *sqlx.DB, humanID string) (Info, error) {
    var person Info
    err := db.Get(&person, "SELECT * FROM people WHERE human_id = $1", humanID)
    return person, err
}

func respondWithError(w http.ResponseWriter, message string, statusCode int, err error) {
    if err != nil {
        log.Printf("DEBUG: %s: %v", message, err)
    } else {
        log.Printf("DEBUG: %s", message)
    }
    http.Error(w, message, statusCode)
}

func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        log.Printf("DEBUG: JSON encode error: %v", err)
    }
}
