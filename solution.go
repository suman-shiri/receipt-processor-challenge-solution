package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/google/uuid"
)

type Receipt struct {
	Retailer     string  `json:"retailer"`
	PurchaseDate string  `json:"purchaseDate"`
	PurchaseTime string  `json:"purchaseTime"`
	Items        []Item  `json:"items"`
	Total        string  `json:"total"`
}

type Item struct {
	ShortDescription string `json:"shortDescription"`
	Price           string `json:"price"`
}

type ResponseID struct {
	ID string `json:"id"`
}

type ResponsePoints struct {
	Points int `json:"points"`
}

var (
	receipts = make(map[string]Receipt)
	points   = make(map[string]int)
	mutex    sync.Mutex
)

// Function to calculate points for a given receipt
func calculatePoints(receipt Receipt) int {
	points := 0

	reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
	alphanumericRetailer := reg.ReplaceAllString(receipt.Retailer, "")
	points += len(alphanumericRetailer)

	total, _ := strconv.ParseFloat(receipt.Total, 64)

	if math.Mod(total, 1) == 0 {
		points += 50
	}

	if math.Mod(total, 0.25) == 0 {
		points += 25
	}

	points += (len(receipt.Items) / 2) * 5

	for _, item := range receipt.Items {
		description := strings.TrimSpace(item.ShortDescription)
		if len(description)%3 == 0 {
			price, _ := strconv.ParseFloat(item.Price, 64)
			points += int(math.Ceil(price * 0.2))
		}
	}

	dateParts := strings.Split(receipt.PurchaseDate, "-")
	if len(dateParts) == 3 {
		day, _ := strconv.Atoi(dateParts[2])
		if day%2 != 0 {
			points += 6
		}
	}

	purchaseTimeParts := strings.Split(receipt.PurchaseTime, ":")
	if len(purchaseTimeParts) == 2 {
		hour, _ := strconv.Atoi(purchaseTimeParts[0])
		minute, _ := strconv.Atoi(purchaseTimeParts[1])
		timeVal := hour*60 + minute
		if timeVal >= 840 && timeVal < 960 {
			points += 10
		}
	}

	return points
}

// Function to extract the uid from the url path
func extractUUID(url string) string {
	re := regexp.MustCompile(`/receipts/([a-f0-9\-]+)/points`)
	match := re.FindStringSubmatch(url)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// Function to validate receipt data
func validateReceipt(receipt Receipt) error {
	if receipt.Retailer == "" || receipt.PurchaseDate == "" || receipt.PurchaseTime == "" || receipt.Total == "" || len(receipt.Items) == 0 {
		return fmt.Errorf("The receipt is invalid.")
	}

	retailerPattern := regexp.MustCompile(`^[\w\s\-&]+$`)
	if !retailerPattern.MatchString(receipt.Retailer) {
		return fmt.Errorf("The receipt is invalid.")
	}

	if _, err := time.Parse("2006-01-02", receipt.PurchaseDate); err != nil {
		return fmt.Errorf("The receipt is invalid.")
	}

	if _, err := time.Parse("15:04", receipt.PurchaseTime); err != nil {
		return fmt.Errorf("The receipt is invalid.")
	}

	totalPricePattern := regexp.MustCompile(`^\d+\.\d{2}$`)
	if !totalPricePattern.MatchString(receipt.Total) {
		return fmt.Errorf("The receipt is invalid.")
	}

	for _, item := range receipt.Items {
		if item.ShortDescription == "" || item.Price == "" {
			return fmt.Errorf("The receipt is invalid.")
		}
		ShortDescriptionPattern := regexp.MustCompile(`^[\w\s\-]+$`)
		if !ShortDescriptionPattern.MatchString(item.ShortDescription) {
			return fmt.Errorf("The receipt is invalid.")
		}
		if !totalPricePattern.MatchString(item.Price) {
			return fmt.Errorf("The receipt is invalid.")
		}
	}
	return nil
}

// Handler to get points for a receipt
func getPointsHandler(w http.ResponseWriter, r *http.Request) {
	id := extractUUID(r.URL.Path)
	mutex.Lock()
	p, exists := points[id]
	mutex.Unlock()

	if !exists {
		http.Error(w, "No receipt found for that ID.", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponsePoints{Points: p})
}

// Handler to process receipts
func processReceiptHandler(w http.ResponseWriter, r *http.Request) {
	var receipt Receipt
	if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
		http.Error(w, "The receipt is invalid.", http.StatusBadRequest)
		return
	}

	if err := validateReceipt(receipt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	points[id] = calculatePoints(receipt)
	mutex.Lock()
	receipts[id] = receipt
	mutex.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ResponseID{ID: id})
}

func main() {
    	http.HandleFunc("/receipts/", getPointsHandler)
	http.HandleFunc("/receipts/process", processReceiptHandler)

	fmt.Println("Server started on port 8080")
	http.ListenAndServe(":8080", nil)
}

