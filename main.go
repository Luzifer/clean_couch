package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/spf13/pflag"
)

var config = struct {
	CouchBaseURL string
	Database     string
	View         string
	Routines     int

	// Private storage
	totalNumberOfDocuments int
	processedDocuments     int
	processChannel         chan bool
	concurrencyChannel     chan bool
}{
	processChannel: make(chan bool, 10),
}

func main() {
	pflag.StringVar(&config.CouchBaseURL, "baseurl", "http://localhost:5984", "BaseURL of your CouchDB instance")
	pflag.StringVar(&config.Database, "database", "", "The database containing your view and the data to delete")
	pflag.StringVar(&config.View, "view", "", "The view selecting the data to delete")
	pflag.IntVar(&config.Routines, "concurrency", 20, "How many delete requests should get processed concurrently?")
	pflag.Parse()

	if config.Database == "" || config.View == "" {
		pflag.Usage()
		return
	}

	delData := struct {
		Rows []struct {
			ID  string `json:"id"`
			Rev string `json:"key"`
		} `json:"rows"`
	}{}

	err := backoff.Retry(func() error {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/%s", config.CouchBaseURL, config.Database, config.View), nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if err := json.NewDecoder(res.Body).Decode(&delData); err != nil {
			return err
		}

		return nil
	}, backoff.NewExponentialBackOff())
	if err != nil {
		fmt.Printf("Tried to get the view but did not succeed: %s", err)
		return
	}

	config.totalNumberOfDocuments = len(delData.Rows)
	config.processedDocuments = 0

	config.concurrencyChannel = make(chan bool, config.Routines)

	go func() {
		for _, row := range delData.Rows {
			// Blocks when concurrency channel is full
			config.concurrencyChannel <- true

			go func(finChan chan bool, conChan chan bool, id, rev string) {
				// Retry deletes
				bo := backoff.NewExponentialBackOff()
				bo.InitialInterval = 5 * time.Second

				err := backoff.Retry(func() error {
					url := fmt.Sprintf("%s/%s/%s?rev=%s", config.CouchBaseURL, config.Database, id, rev)
					req, _ := http.NewRequest("DELETE", url, nil)
					res, err := http.DefaultClient.Do(req)
					if err != nil {
						return err
					}
					res.Body.Close()
					return nil
				}, bo)
				if err != nil {
					fmt.Printf("Unable to delete document with ID %s", id)
				}
				// Increase finished counter
				finChan <- true

				// Remove self from concurrency limit
				<-conChan
			}(config.processChannel, config.concurrencyChannel, row.ID, row.Rev)
		}
	}()

	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-config.processChannel:
			config.processedDocuments++
			if config.processedDocuments == config.totalNumberOfDocuments {
				fmt.Print("\n\n")
				return
			}
		case <-ticker.C:
			fmt.Printf("Processed %d of %d documents.\r", config.processedDocuments, config.totalNumberOfDocuments)
		}
	}

}
