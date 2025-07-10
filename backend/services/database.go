package services

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetArticleFromDBService queries db-service for article by URL
func GetArticleFromDBService(url string) (string, error) {
	dbServiceURL := "http://db-service:5000/article?url=" + url
	resp, err := http.Get(dbServiceURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("not found")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// SaveArticleToDBService saves article to db-service
func SaveArticleToDBService(articleJSON string) error {
	fmt.Printf("\nCalled saveArticleToDBService\n")
	fmt.Printf("Request body: %s\n", articleJSON)

	dbServiceURL := "http://db-service:5000/article"
	resp, err := http.Post(dbServiceURL, "application/json", bytes.NewBuffer([]byte(articleJSON)))
	if err != nil {
		fmt.Printf("Error saving article to db-service: %v\n", err)
		return err
	} else {
		fmt.Printf("response: %v\n", resp)
		fmt.Printf("Article saved to db-service: %v\n", resp.StatusCode)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("Failed to save article: %v\n", resp.StatusCode)
		// Read response body for more details
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Response body: %s\n", string(body))
		return fmt.Errorf("failed to save article")
	}
	return nil
} 