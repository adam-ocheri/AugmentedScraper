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

// UpdateConversationInDBService updates conversation for an article in db-service
func UpdateConversationInDBService(uuid string, conversationJSON string) error {
	fmt.Printf("Updating conversation for UUID: %s\n", uuid)
	fmt.Printf("Request body: %s\n", conversationJSON)

	dbServiceURL := "http://db-service:5000/article/conversation"
	req, err := http.NewRequest("PUT", dbServiceURL, bytes.NewBuffer([]byte(conversationJSON)))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error updating conversation in db-service: %v\n", err)
		return err
	}
	defer resp.Body.Close()
	
	fmt.Printf("Conversation update response: %v\n", resp.StatusCode)
	
	if resp.StatusCode != http.StatusOK {
		// Read response body for more details
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Response body: %s\n", string(body))
		return fmt.Errorf("failed to update conversation: %d", resp.StatusCode)
	}
	
	fmt.Printf("Conversation updated successfully for UUID: %s\n", uuid)
	return nil
}

// GetArticleByUUIDFromDBService queries db-service for article by UUID
func GetArticleByUUIDFromDBService(uuid string) (string, error) {
	dbServiceURL := "http://db-service:5000/article/uuid/" + uuid
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