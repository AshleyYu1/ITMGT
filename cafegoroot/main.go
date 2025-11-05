package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type IndexPageData struct {
	Username string
	Products []Product
}

type CartPageData struct {
	CartItems []CartItem
	User      User
}

func generateSessionToken() string {
	rawBytes := make([]byte, 16)
	_, err := rand.Read(rawBytes)
	if err != nil {
		log.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(rawBytes)
}

func cartHandler(w http.ResponseWriter, r *http.Request) {
	cookies := r.Cookies()
	var sessionToken string
	for _, cookie := range cookies {
		if cookie.Name == "cafego_session" {
			sessionToken = cookie.Value
			break
		}
	}
	user := getUserFromSessionToken(sessionToken)

	if r.Method == "GET" {
		cartItems := getCartItemsByUser(user)
		tmpl, err := template.ParseFiles("./templates/cart.html")
		if err != nil {
			log.Fatal(err)
		}
		pageData := CartPageData{
			CartItems: cartItems,
			User:      user,
		}
		tmpl.Execute(w, pageData)

	} else if r.Method == "POST" {
		cartItems := getCartItemsByUser(user)
		res, err := database.Exec("INSERT INTO cgo_transaction (user_id, created_at) VALUES (?, datetime('now'))", user.Id)
		if err != nil {
			log.Fatal(err)
		}
		transactionID, err := res.LastInsertId()
		if err != nil {
			log.Fatal(err)
		}
		for _, ci := range cartItems {
			_, err := database.Exec("INSERT INTO cgo_line_item (transaction_id, product_id, quantity) VALUES (?, ?, ?)", transactionID, ci.ProductId, ci.Quantity)
			if err != nil {
				log.Fatal(err)
			}
		}
		_, err = database.Exec("DELETE FROM cgo_cart_item WHERE user_id = ?", user.Id)
		if err != nil {
			log.Fatal(err)
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func getCartItemsByUser(user User) []CartItem {
	userId := user.Id
	q := `
	SELECT
		cgo_cart_item.rowid,
		cgo_cart_item.user_id,
		cgo_cart_item.product_id,
		cgo_cart_item.quantity,
		cgo_product.name
	FROM cgo_cart_item
	LEFT JOIN cgo_product ON cgo_cart_item.product_id = cgo_product.rowid
	WHERE cgo_cart_item.user_id = ?
	`
	rows, err := database.Query(q, userId)
	if err == sql.ErrNoRows {
		return []CartItem{}
	} else if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var result []CartItem
	for rows.Next() {
		var cartItem CartItem
		rows.Scan(&cartItem.Id, &cartItem.UserId, &cartItem.ProductId, &cartItem.Quantity, &cartItem.ProductName)
		result = append(result, cartItem)
	}
	return result
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Get the product ID
		reqPath := r.URL.Path
		splitPath := strings.Split(reqPath, "/")
		elemCount := len(splitPath)
		// Do note that this will be a string.
		productId := splitPath[elemCount-1]
		// Need to convert from string to int
		intId, err := strconv.Atoi(productId)
		if err != nil {
			log.Fatal(err)
		}
		// Predeclare a product
		var product Product
		// Check each product for whether it matches the given ID
		for _, p := range getProducts() {
			if p.Id == intId {
				product = p
				break
			}
		}
		// If the for loop failed, then product will be the "zero-value" of the Product struct
		if product == (Product{}) {
			log.Fatal("Can't find product with that ID")
		}
		// Template rendering
		tmpl, err := template.ParseFiles("./templates/product.html")
		if err != nil {
			log.Fatal(err)
		}
		err = tmpl.Execute(w, product)
		if err != nil {
			log.Fatal(err)
		}
	} else if r.Method == "POST" {

		cookies := r.Cookies()
		var sessionToken string
		for _, cookie := range cookies {
			if cookie.Name == "cafego_session" {
				sessionToken = cookie.Value
				break
			}
		}
		user := getUserFromSessionToken(sessionToken)
		userId := user.Id
		// Get product ID
		sProductId := r.FormValue("product_id")
		productId, err := strconv.Atoi(sProductId)
		if err != nil {
			log.Fatal(err)
		}
		// Get quantity
		sQuantity := r.FormValue("quantity")
		quantity, err := strconv.Atoi(sQuantity)
		if err != nil {
			log.Fatal(err)
		}
		// Echo values
		// Create a cart item
		createCartItem(userId, productId, quantity)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	var sessionToken string
	for _, cookie := range r.Cookies() {
		if cookie.Name == "cafego_session" {
			sessionToken = cookie.Value
			break
		}
	}
	user := getUserFromSessionToken(sessionToken)
	sampleProducts := getProducts()
	pageData := IndexPageData{Username: user.Username, Products: sampleProducts}
	tmpl, err := template.ParseFiles("./templates/index.html")
	if err != nil {
		log.Fatal(err)
	}
	tmpl.Execute(w, pageData)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("./templates/login.html")
		if err != nil {
			log.Fatal(err)
		}
		tmpl.Execute(w, nil)
		return
	}

	rUsername := r.FormValue("username")
	rPassword := r.FormValue("password")
	var user User
	for _, u := range getUsers() {
		if rUsername == u.Username && rPassword == u.Password {
			user = u
			break
		}
	}

	if user == (User{}) {
		fmt.Fprint(w, "Invalid login. Please go back and try again.")
		return
	}

	token := generateSessionToken()
	setSession(token, user)
	cookie := http.Cookie{Name: "cafego_session", Value: token, Path: "/"}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

func main() {
	initDB()
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/product/", productHandler)
	http.HandleFunc("/login/", loginHandler)
	log.Println("Server starting on http://localhost:5000")
	http.HandleFunc("/cart/", cartHandler)
	http.ListenAndServe(":5000", nil)
}
