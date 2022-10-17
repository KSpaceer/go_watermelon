package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// addUserCall is used to call (through gRPC) AddUser method on main service.
func addUserCall(nickname, email, mainServiceLocation string) (string, error) {
	user := struct {
		Nickname string `json:"nickname,omitempty"`
		Email    string `json:"email,omitempty"`
	}{}
	user.Nickname, user.Email = nickname, email
	jsonData, err := json.Marshal(&user)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, mainServiceLocation+"/v1/users", bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyData)
	if resp.StatusCode > 399 {
		return "", fmt.Errorf("Got response status %q with body %q", resp.Status, bodyStr)
	}
	return bodyStr, nil
}

// deleteUserCall is used to call (through gRPC) DeleteUser method on main service.
func deleteUserCall(nickname, mainServiceLocation string) (string, error) {
	req, err := http.NewRequest(http.MethodDelete, mainServiceLocation+"/v1/users/"+nickname, nil)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyData)
	if resp.StatusCode > 399 {
		return "", fmt.Errorf("Got response status %q with body %q", resp.Status, bodyStr)
	}
	return bodyStr, nil
}

// deleteUserCall is used to call (through gRPC) DeleteUser method on main service.
func listUsersCall(mainServiceLocation string) (string, error) {
	resp, err := http.Get(mainServiceLocation + "/v1/users")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyData)
	if resp.StatusCode > 399 {
		return "", fmt.Errorf("Got response status %q with body %q", resp.Status, bodyStr)
	}
	return bodyStr, nil
}
