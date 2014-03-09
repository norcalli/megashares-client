package main

import (
	"encoding/json"
	"fmt"
	"github.com/cheggaaa/pb"
	"github.com/norcalli/megashares"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

const credentialFile = "login.json"

type Credentials struct {
	Username string
	Password string
}

// TODO: File cookiejar
func main() {
	creds, err := loadCredentials()
	if err != nil {
		fmt.Printf("Failed to load credentials. (%s)\n", err)
		return
	}
	if len(os.Args) == 1 {
		fmt.Printf("Usage: %s <query>\n", os.Args[0])
		return
	}
	// Form a query by joining all the arguments.
	query := strings.Join(os.Args[1:], " ")

	client := megashares.New()
	// Attempt to login.
	for {
		if err := client.Login(creds.Username, creds.Password); err != nil {
			fmt.Printf("Failed to login: %s\n", err)
			creds = askForCredentials()
			// log.Fatalf("Couldn't login! Reason: %s\n", err)
		} else {
			break
		}
	}

	// Perform the search
	entries, _ := client.SearchEntries(query)

	// Print out the results of the search for the user to pick from.
	for i, entry := range entries {
		fmt.Printf("%d: %s\n", i, entry.Filename)
	}

	// Get a valid number to choose from from the input loop.
	// TODO: Allow for pagination by returning (choice, page).
	choice := getValidNumber(0, len(entries)-1)
	entry := entries[choice]

	if file, response, err := ContinueDownload(client.Client, entry.Filename, entry.Url); err != nil {
		log.Fatal(err)
	} else {
		defer file.Close()
		defer response.Body.Close()
		length := response.ContentLength
		// Initialize progress bar.
		bar := pb.StartNew(int(length)).SetUnits(pb.U_BYTES)
		bar.ShowSpeed = true
		writer := io.MultiWriter(file, bar)
		io.Copy(writer, response.Body)
	}
}

func updateCredentials(creds *Credentials) error {
	if file, err := os.Create(credentialFile); err != nil {
		return err
	} else {
		return json.NewEncoder(file).Encode(creds)
	}
}

func askForCredentials() *Credentials {
	creds := &Credentials{}
	// TODO: Error handling?
	fmt.Print("Enter username: ")
	fmt.Scanf("%s\n", &creds.Username)
	fmt.Print("Enter password: ")
	fmt.Scanf("%s\n", &creds.Password)
	updateCredentials(creds)
	return creds
}

func loadCredentials() (*Credentials, error) {
	if file, err := os.Open("login.json"); err != nil {
		return askForCredentials(), nil
	} else {
		creds := &Credentials{}
		if err := json.NewDecoder(file).Decode(creds); err != nil {
			return nil, err
		}
		return creds, nil
	}
}

func getValidNumber(lower, upper int) int {
	i := 0
	for {
		fmt.Print("Enter a number: ")
		_, err := fmt.Scanf("%d", &i)

		if err != nil || i < lower || i > upper {
			fmt.Printf("Invalid: enter a number between %d and %d\n", lower, upper)
		} else {
			return i
		}
	}
}

func ContinueDownload(client *http.Client, filename, url string) (*os.File, *http.Response, error) {
	// See if file already exists
	// if file, err := os.Open(entry.Filename); err != nil {
	if file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND, 0660); err != nil {
		// File doesn't exist, attempt to get new download.
		if response, err := client.Get(url); err != nil {
			return nil, nil, err
		} else {
			// Create new file.
			if file, err := os.Create(filename); err != nil {
				response.Body.Close()
				return nil, nil, err
			} else {
				return file, response, nil
			}
		}
	} else {
		// File does exist, attempt to continue download.
		if request, err := http.NewRequest("GET", url, nil); err != nil {
			return nil, nil, err
		} else {
			var start int64
			if fileInfo, err := file.Stat(); err != nil {
				// file.Close() // Neccessary?
				return nil, nil, err
			} else {
				start = fileInfo.Size()
			}
			request.Header.Add("Range", fmt.Sprintf("bytes=%d-", start))
			if response, err := client.Do(request); err != nil {
				// file.Close()
				return nil, nil, err
			} else {
				return file, response, nil
			}
			// file.Close()
			// file, err = os.OpenFile(entry.Filename, os.O_WRONLY|os.O_APPEND, 0660)
		}
	}
}
