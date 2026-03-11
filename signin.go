package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/fereidani/httpdecompressor"
	"github.com/joho/godotenv"
)

func (cfg *apiConfig) rpiSignIn() bool {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("failed to load .env file")
	}

	rpiEmail := os.Getenv("RPI_EMAIL")
	rpiPW := os.Getenv("RPI_PW")

	// get sign-in information, grep authority and hidden to make sign-in obj
	r, err := http.NewRequest("GET", "https://id.raspberrypi.com/sign-in", nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to get sign-in information: received %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	authToken := getAuth(string(body), "hidden", true)
	signIn := SignIn{
		AuthToken: authToken, 
		Email: rpiEmail, 
		Password: rpiPW, 
		Commit: "Sign in",
	}
	signInValues := url.Values{}
	signInValues.Set("authenticity_token", signIn.AuthToken)
	signInValues.Set("email", signIn.Email)
	signInValues.Set("password", signIn.Password)
	signInValues.Set("commit", signIn.Commit)
	signInData := signInValues.Encode()
	fmt.Printf("\n%s\n\n", signInData)

	// post obj to /session, grep authority and hidden, and get OTP to make verify obj
	r, err = http.NewRequest(
		"POST", 
		"https://id.raspberrypi.com/session", 
		bytes.NewBuffer([]byte(signInData)),
	)
	if err != nil {
		log.Fatal(err)
	}
	setHeader(
		r, 
		"application/x-www-form-urlencoded", 
		"https://id.raspberrypi.com", 
		"https://id.raspberrypi.com/sign-in",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode == 400 || resp.StatusCode > 401 {
		log.Fatalf("Failed to sign into rpi account: received %s", resp.Status)
	}
	body, err = httpdecompressor.ReadAll(resp)
	if err != nil {
		log.Fatal(err)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("2FA: ")
	otp, _ := reader.ReadString('\n')

	authToken = getAuth(string(body), "hidden", true)
	verify := Verify{
		AuthToken: authToken, 
		OTP: strings.TrimSpace(otp), 
		Commit: "Verify and sign in",
	}
	verifyValues := url.Values{}
	verifyValues.Set("authenticity_token", verify.AuthToken)
	verifyValues.Set("otp", verify.OTP)
	verifyValues.Set("commit", verify.Commit)
	verifyData := verifyValues.Encode()

	// post obj to /session/verify, then follow through location in resp
	r, err = http.NewRequest(
		"POST",
		"https://id.raspberrypi.com/session/verify", 
		bytes.NewBuffer([]byte(verifyData)),
	)
	if err != nil {
		log.Fatal(err)
	}
	setHeader(
		r,
		"application/x-www-form-urlencoded",
		"https://id.raspberrypi.com",
		"https://id.raspberrypi.com/session",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		log.Fatal(err)
	} else if resp.StatusCode >= 400 {
		log.Fatalf("Failed to verify the rpi account: received %s", resp.Status)
	}
	return true
}
