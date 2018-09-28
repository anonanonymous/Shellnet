package main

type jsonResponse struct {
	Status string
	Data   map[string]interface{}
}

type transaction struct {
	Destination string
	Hash        string
	Amount      string
	Date        string
	PaymentID   string
	ID          string
}
