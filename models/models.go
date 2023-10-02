package models

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Purple = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

type Pair struct {
	Title string  `json:"title"`
	Value float64 `json:"value"`
}

type SchalterStatus struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	WidgetId string `json:"widgetId"`
	Locked   int    `json:"locked"`
}

type QueryResponse struct {
	LineSets [][]Pair `json:"lineSets"`
	Names    []string `json:"names"`
	Amount   int      `json:"amount"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AuthResponse struct {
	SessionToken string `json:"session_token"`
	FirstName    string `json:"firstname"`
	LastName     string `json:"lastname"`
	Email        string `json:"email"`
}

type RegisterResponse struct {
	Success bool `json:"success"`
}
