package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	// "regexp"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"

	// _ "github.com/jackc/pgx/v4"
	// _ "github.com/jackc/pgx/v4/pgxpool"

	// "cool-api/route"

	"github.com/redis/go-redis/v9"
)

// User struct
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Surname   string    `json:"surname"`
	Floor     int       `json:"floor"`
	Status    string    `json:"status"`
	DOB       time.Time `json:"dob"`
	DateAdded time.Time `json:"date_added"`
}

// Users array
type Users []User

// Database connection
var db *pgx.Conn

// Redis connection
var client *redis.Client

func main() {
	// Connect to Postgres
	var err error
	db, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(context.Background())

	// Connect to Redis
	// client, err = redis.Dial("tcp", os.Getenv("REDIS_URL"))
	client = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"), // password set
		DB:       0,                           // use default DB
	})

	// Check connection
	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		fmt.Println("Error connecting to Redis")
		os.Exit(2)
	}

	fmt.Println("Connected to Redis")
	defer client.Close()

	// Create router
	r := mux.NewRouter()

	// Create routes
	r.HandleFunc("/users", getUsers).Methods("GET")
	r.HandleFunc("/users", createUser).Methods("POST")
	r.HandleFunc("/users/{id}", getUser).Methods("GET")
	r.HandleFunc("/users/{id}", updateUser).Methods("PUT")
	r.HandleFunc("/users/{id}", deleteUser).Methods("DELETE")
	r.HandleFunc("/users/import", importUsers).Methods("POST")

	// Start server
	log.Fatal(http.ListenAndServe(":8000", r))
}

// Get all users
func getUsers(w http.ResponseWriter, r *http.Request) {
	// Get users from Redis
	key := "users"
	reply, err := client.Get(context.Background(), key).Result()
	if err == nil {
		// Return users from Redis
		fmt.Fprintf(w, reply)
		return
	}

	// Get users from Postgres
	users := Users{}
	rows, err := db.Query(context.Background(), "SELECT * FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		user := User{}
		err := rows.Scan(&user.ID, &user.Name, &user.Surname, &user.Floor, &user.Status, &user.DOB, &user.DateAdded)
		if err != nil {
			log.Fatal(err)
		}
		users = append(users, user)
	}

	// Set users in Redis
	json_r, err := json.Marshal(users)
	if err != nil {
		log.Fatal(err)
	}
	client.Do(context.Background(), "get", key, json_r)

	// Return users
	json.NewEncoder(w).Encode(users)
}

// Create user
func createUser(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal request body
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		log.Fatal(err)
	}

	// Insert user into Postgres
	sqlStatement := `
		INSERT INTO users (name, surname, floor, status, dob, date_added)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	err = db.QueryRow(context.Background(), sqlStatement, user.Name, user.Surname, user.Floor, user.Status, user.DOB, user.DateAdded).Scan(&user.ID)
	if err != nil {
		log.Fatal(err)
	}

	// Return user
	json.NewEncoder(w).Encode(user)
}

// Get user
func getUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		log.Fatal(err)
	}

	// Get user from Postgres
	user := User{}
	err = db.QueryRow(context.Background(), "SELECT * FROM users WHERE id=$1", id).Scan(&user.ID, &user.Name, &user.Surname, &user.Floor, &user.Status, &user.DOB, &user.DateAdded)
	if err != nil {
		log.Fatal(err)
	}

	// Return user
	json.NewEncoder(w).Encode(user)
}

// Update user
func updateUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		log.Fatal(err)
	}

	// Read request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal request body
	user := User{}
	err = json.Unmarshal(body, &user)
	if err != nil {
		log.Fatal(err)
	}

	// Update user in Postgres
	sqlStatement := `
		UPDATE users
		SET name = $2, surname = $3, floor = $4, status = $5, dob = $6
		WHERE id = $1;`
	_, err = db.Exec(context.Background(), sqlStatement, id, user.Name, user.Surname, user.Floor, user.Status, user.DOB)
	if err != nil {
		log.Fatal(err)
	}

	// Return user
	json.NewEncoder(w).Encode(user)
}

// Delete user
func deleteUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		log.Fatal(err)
	}

	// Delete user from Postgres
	sqlStatement := `DELETE FROM users WHERE id = $1;`
	_, err = db.Exec(context.Background(), sqlStatement, id)
	if err != nil {
		log.Fatal(err)
	}

	// Return success
	json.NewEncoder(w).Encode("User deleted")
}

// Import users from XLS\XLSX file
func importUsers(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshal request body
	users := Users{}
	err = json.Unmarshal(body, &users)
	if err != nil {
		log.Fatal(err)
	}

	// Insert users into Postgres
	sqlStatement := `
		INSERT INTO users (name, surname, floor, status, dob, date_added)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`
	for _, user := range users {
		err = db.QueryRow(context.Background(), sqlStatement, user.Name, user.Surname, user.Floor, user.Status, user.DOB, user.DateAdded).Scan(&user.ID)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Return users
	json.NewEncoder(w).Encode(users)
}
