package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/CraigYanitski/rpi-cli/internal/utils"
	"github.com/fereidani/httpdecompressor"
	"github.com/joho/godotenv"
	"golang.org/x/term"
)

type SignIn struct {
	AuthToken  string  `json:"authenticity_token"`
	Email      string  `json:"email"`
	Password   string  `json:"password"`
	Commit     string  `json:"commit"`
}

type Verify struct {
	AuthToken  string  `json:"authenticity_token"`
	OTP        string  `json:"otp"`
	Commit     string  `json:"commit"`
}

func (cfg *apiConfig) rpiSignIn() error {
	// check if values were given in CLI command
	email := flag.String("email", "", "raspberry pi connect account email")
	pw := flag.String("password", "", "raspberry pi connect account password")
	flag.Parse()

	fmt.Println("args", *email, *pw)

	// check if session_id cookie exists (this is long-lived and used to bypass the signin step)
	idCookies, err := utils.GetCookieNames(cfg.client.Jar, "https://id.raspberrypi.com")
	if err != nil {
		return err
	}
	isSignedIn := slices.Contains(idCookies, "session_id")
	if isSignedIn {
		return nil
	}

	var rpiEmail = ""
	var rpiPW = ""
	var input = []byte{}

	homeDir, _ := os.UserHomeDir()
	_, err = os.Stat(homeDir + "/.rpi-cli/.env")
	if (*email != "") && (*pw != "") {
		rpiEmail = *email
		rpiPW = *pw
	} else if err != nil {
		cfg.spinner.Clear()
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Email: ")
		input, _ = reader.ReadBytes('\n')
		rpiEmail = strings.TrimSpace(string(input))
		fmt.Print("Password: ")
		input, _ = term.ReadPassword(int(syscall.Stdin))
		rpiPW = strings.TrimSpace(string(input))
		fmt.Println("")
		cfg.spinner.Start()
	} else {
		err = godotenv.Load(homeDir + "/.rpi-cli/.env")
		if err != nil {
			return errors.New("failed to load .env file")
		}

		rpiEmail = os.Getenv("RPI_EMAIL")
		rpiPW = os.Getenv("RPI_PW")
	}


	// get sign-in information, grep authority and hidden to make sign-in obj
	r, err := http.NewRequest("GET", "https://id.raspberrypi.com/sign-in", nil)
	if err != nil {
		return err
	}

	resp, err := cfg.client.Do(r)
	if err != nil {
		return err
	} else if resp.StatusCode >= 400 {
		return fmt.Errorf("Failed to get sign-in information: received %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	authToken := utils.GetAuth(string(body), "hidden", false)
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
	//fmt.Println(signInData)

	// post obj to /session, grep authority and hidden, and get OTP to make verify obj
	r, err = http.NewRequest(
		"POST", 
		"https://id.raspberrypi.com/session", 
		bytes.NewBuffer([]byte(signInData)),
	)
	if err != nil {
		return err
	}
	setHeader(
		r, 
		"application/x-www-form-urlencoded", 
		"https://id.raspberrypi.com", 
		"https://id.raspberrypi.com/sign-in",
	)

	resp, err = cfg.client.Do(r)
	if err != nil {
		return err
	} else if resp.StatusCode == 400 || resp.StatusCode > 401 {
		return fmt.Errorf("Failed to sign into rpi account: received %s", resp.Status)
	}
	body, err = httpdecompressor.ReadAll(resp)
	if err != nil {
		return err
	}

	// wait for two-factor authentification if t is enabled
	if resp.Request.URL.String() != "https://id.raspberrypi.com/profile" {
		// wait for user to enter one time password
		cfg.spinner.Clear()
		fmt.Print("2FA: ")
		otpBytes, _ := term.ReadPassword(int(syscall.Stdin))
		otp := strings.TrimSpace(string(otpBytes))
		fmt.Println("")
		cfg.spinner.Start()

		authToken = utils.GetAuth(string(body), "hidden", false)
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
			return err
		}
		setHeader(
			r,
			"application/x-www-form-urlencoded",
			"https://id.raspberrypi.com",
			"https://id.raspberrypi.com/session",
		)

		resp, err = cfg.client.Do(r)
		if err != nil {
			return err
		} else if resp.StatusCode >= 400 {
			return fmt.Errorf("Failed to verify the rpi account: received %s", resp.Status)
		}
	}
	return nil
}
