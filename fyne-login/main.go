package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// User represents a user in the database
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Initialize the SQLite database
func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// Print working directory
	dir, _ := os.Getwd()
	fmt.Println("Working directory:", dir)

	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
		"id" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		"username" TEXT UNIQUE,
		"password" TEXT
	);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}

	fmt.Println("Database initialized")
}

// Authenticate checks if the username and password match a user in the database
func Authenticate(username, password string) bool {
	var dbPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&dbPassword)
	if err != nil {
		log.Println("Authentication failed:", err)
		return false
	}
	return password == dbPassword
}

// ShowLoginWindow creates and displays the login window
func ShowLoginWindow(myApp fyne.App) {
	myWindow := myApp.NewWindow("Login Page")
	myWindow.Resize(fyne.NewSize(400, 300)) // Increased window size

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Enter username...")

	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Enter password...")

	loginButton := widget.NewButton("Login", func() {
		username := usernameEntry.Text
		password := passwordEntry.Text

		if Authenticate(username, password) {
			dialog.ShowInformation("Login Successful", "Welcome, "+username, myWindow)
			ShowCRUDWindow(myApp) // Open the CRUD window after successful login
		} else {
			dialog.ShowError(fmt.Errorf("Invalid username or password"), myWindow)
		}
	})

	content := container.NewVBox(
		widget.NewLabel("Username:"),
		usernameEntry,
		widget.NewLabel("Password:"),
		passwordEntry,
		loginButton,
	)

	myWindow.SetContent(content)
	myWindow.Show()
}

func ShowCRUDWindow(myApp fyne.App) {
	crudWindow := myApp.NewWindow("CRUD Operations")
	crudWindow.Resize(fyne.NewSize(800, 600)) // Increased window size

	// Create User
	createUsernameEntry := widget.NewEntry()
	createUsernameEntry.SetPlaceHolder("Enter username...")
	createPasswordEntry := widget.NewPasswordEntry()
	createPasswordEntry.SetPlaceHolder("Enter password...")
	createButton := widget.NewButton("Create User", func() {
		user := User{
			Username: createUsernameEntry.Text,
			Password: createPasswordEntry.Text,
		}
		jsonData, _ := json.Marshal(user)
		resp, err := http.Post("http://localhost:8080/user", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to create user"), crudWindow)
			return
		}
		defer resp.Body.Close()
		dialog.ShowInformation("Success", "User created successfully", crudWindow)
	})

	// Table to display users
	userTable := widget.NewTable(
		func() (int, int) {
			return 0, 3 // 3 columns: ID, Username, Password
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			label.SetText("") // Clear previous content
		},
	)

	// Set column widths for better readability
	userTable.SetColumnWidth(0, 50)  // ID column
	userTable.SetColumnWidth(1, 150) // Username column
	userTable.SetColumnWidth(2, 150) // Password column

	// Function to refresh the user table
	refreshUserTable := func() {
		resp, err := http.Get("http://localhost:8080/users")
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to fetch users"), crudWindow)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var users []User
		json.Unmarshal(body, &users)

		// Update the table size and data
		userTable.Length = func() (int, int) {
			return len(users) + 1, 3 // +1 for the header row
		}
		userTable.UpdateCell = func(i widget.TableCellID, o fyne.CanvasObject) {
			label := o.(*widget.Label)
			if i.Row == 0 {
				// Header row
				switch i.Col {
				case 0:
					label.SetText("ID")
				case 1:
					label.SetText("Username")
				case 2:
					label.SetText("Password")
				}
			} else {
				// Data rows
				user := users[i.Row-1]
				switch i.Col {
				case 0:
					label.SetText(fmt.Sprintf("%d", user.ID))
				case 1:
					label.SetText(user.Username)
				case 2:
					label.SetText(user.Password)
				}
			}
		}
		userTable.Refresh()
	}

	// Refresh button
	refreshButton := widget.NewButton("Refresh", refreshUserTable)

	// Update User
	updateIDEntry := widget.NewEntry()
	updateIDEntry.SetPlaceHolder("Enter user ID...")
	updatePasswordEntry := widget.NewPasswordEntry()
	updatePasswordEntry.SetPlaceHolder("Enter new password...")
	updateButton := widget.NewButton("Update User", func() {
		id, err := strconv.Atoi(updateIDEntry.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Invalid ID"), crudWindow)
			return
		}

		user := User{
			Password: updatePasswordEntry.Text,
		}
		jsonData, _ := json.Marshal(user)
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:8080/user/%d", id), bytes.NewBuffer(jsonData))
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to update user"), crudWindow)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to update user"), crudWindow)
			return
		}
		defer resp.Body.Close()

		dialog.ShowInformation("Success", "User updated successfully", crudWindow)
		refreshUserTable() // Refresh the table after updating
	})

	// Delete User
	deleteIDEntry := widget.NewEntry()
	deleteIDEntry.SetPlaceHolder("Enter user ID...")
	deleteButton := widget.NewButton("Delete User", func() {
		id, err := strconv.Atoi(deleteIDEntry.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Invalid ID"), crudWindow)
			return
		}

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:8080/user/%d", id), nil)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to delete user"), crudWindow)
			return
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to delete user"), crudWindow)
			return
		}
		defer resp.Body.Close()

		dialog.ShowInformation("Success", "User deleted successfully", crudWindow)
		refreshUserTable() // Refresh the table after deleting
	})

	// Layout for CRUD window
	crudContent := container.NewVBox(
		widget.NewLabel("Create User"),
		createUsernameEntry,
		createPasswordEntry,
		createButton,
		widget.NewLabel("Users List"),
		container.NewBorder(nil, refreshButton, nil, nil, container.NewScroll(userTable)),
		widget.NewLabel("Update User"),
		updateIDEntry,
		updatePasswordEntry,
		updateButton,
		widget.NewLabel("Delete User"),
		deleteIDEntry,
		deleteButton,
	)

	crudWindow.SetContent(crudContent)
	crudWindow.Show()

	// Refresh the table on window load
	refreshUserTable()
}

// API Handlers

// getUsers returns all users from the database
func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, password FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Password); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// createUser adds a new user to the database
func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", user.Username, user.Password)
	if err != nil {
		log.Println("Failed to insert user:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	user.ID = int(id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// updateUser updates a user's password
func updateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Path[len("/user/"):])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE users SET password = ? WHERE id = ?", user.Password, id)
	if err != nil {
		log.Println("Failed to update user:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteUser deletes a user from the database
func deleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.URL.Path[len("/user/"):])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		log.Println("Failed to delete user:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// setupAPIs configures the API routes and starts the HTTP server
func setupAPIs() {
	http.HandleFunc("/users", getUsers)
	http.HandleFunc("/user", createUser)
	http.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			updateUser(w, r)
		case http.MethodDelete:
			deleteUser(w, r)
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Main function
func main() {
	// Initialize the database
	initDB()

	// Start the API server in a goroutine
	go setupAPIs()

	// Create and run the Fyne GUI application
	myApp := app.New()
	ShowLoginWindow(myApp)
	myApp.Run()
}
