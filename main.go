package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	firebase "firebase.google.com/go"
	"github.com/go-resty/resty/v2"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	bot             *tb.Bot
	credentials     string
	app             *firebase.App
	storageClient   *storage.Client
	firestoreClient *firestore.Client
	ctx             = context.Background()
	projectID       = "alpaca-72130.appspot.com"
	restClient      *resty.Client
)

func init() {
	var err error
	app, err = firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalf("Error initializing app: %v\n", err)
	}
	storageClient, err = storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Error initializing storage client: %v\n", err)
	}

	firestoreClient, err = firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Error initializing firestore client: %v\n", err)
	}
	bot, err = tb.NewBot(tb.Settings{
		// You can also set custom API URL.
		// If field is empty it equals to "https://api.telegram.org".
		// URL: "http://195.129.111.17:8012",

		Token:  "1373983436:AAH0e6ZzWlbgrdtGXZY2hemDVXmZzIkojxc",
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}
}

func main() {

	bot.Handle("/hello", func(m *tb.Message) {
		bot.Send(m.Sender, "Hello World!")
	})

	bot.Handle(tb.OnUserJoined, createUser)
	bot.Handle(tb.OnVoice, voiceEndpoint)
	log.Println("Handlers initialised")
	bot.Start()
}

func createUser(m *tb.Message) {
	bot.Send(m.Sender, "Welcome to Gecko!")
}

func getSentiment(path string) (*string, error) {
	sentiment := new(SentimentResponse)
	_, err := restClient.R().
		SetBody([]byte(fmt.Sprintf(`{"user_id":"684756o837g", "path":"%s"}`, path))).
		SetResult(sentiment).
		Post("10.12.4.127:8000/")
	if err != nil {
		return nil, err
	}
	result := sentiment.Sentiment
	return &result, nil

}

func voiceEndpoint(m *tb.Message) {
	/**
	d, err := json.Marshal(m)
	if err != nil {
		log.Fatal(err)
	}
	*/
	log.Println("Received voice note")
	bucket := storageClient.Bucket(projectID)

	url, err := bot.FileURLByID(m.Voice.FileID)
	if err != nil {
		log.Printf("Error getting file url:  %v", err)
	}
	fileName := fmt.Sprintf("%d.ogg", m.Unixtime)
	filePath := fmt.Sprintf("%s", fileName)

	if err = downloadFile(filePath, url); err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("os.Open: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	bucketRef := fmt.Sprintf("audio_recordings/%s", fileName)
	// Upload an object with storage.Writer.
	wc := bucket.Object(bucketRef).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		log.Fatalf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		log.Fatalf("Writer.Close: %v", err)
	}
	if err = os.Remove(filePath); err != nil {
		log.Fatal("Error removing file")
	}
	sentiment, err := getSentiment(bucketRef)
	if err != nil {
		log.Printf("Error getting sentiment: %v", err)
		return
	}
	log.Printf("Blob %s uploaded.\n", fileName)
	bot.Send(m.Sender, "Detected sentiment: %s", sentiment)

}

func updateDocument(userID string, command string, amount float64) error {
	userDoc := firestoreClient.Doc(fmt.Sprintf("users/%s", userID))
	userSnap, err := userDoc.Get(ctx)
	if err != nil {
		log.Fatalf("Could not get user from firestore: %v", err)
	}

	user := new(User)
	if err := userSnap.DataTo(&user); err != nil {
		log.Fatalf("Could not decode user from firestore: %v", err)
	}
	switch command {
	case "add":
		_, err = userDoc.Set(ctx, User{
			Balance: user.Balance + amount,
		})
		if err != nil {
			log.Fatalf("Error adding balance to user %s: %v", userID, err)
		}
		return nil
	case "sub":
		_, err = userDoc.Set(ctx, User{
			Balance: user.Balance - amount,
		})
		if err != nil {
			log.Fatalf("Error adding balance to user %s: %v", userID, err)
		}
		return nil
	default:
		return errors.New("Invalid command")
	}
}

func downloadFile(filepath string, url string) (err error) {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
