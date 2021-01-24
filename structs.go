package main

type User struct {
	Balance float64 `firestore:"balance"` // in millions
}

type SentimentResponse struct {
	UserID    string `json:"user_id"`
	Sentiment string `json:"sentiment"`
}
