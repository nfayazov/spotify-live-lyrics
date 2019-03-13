package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/rhnvrm/lyric-api-go"
	"github.com/zmb3/spotify"
	"html/template"
	"log"
	"net/http"
	"os"
)

const redirectURI = "http://localhost:8080/callback"

const (
	port = 8080
	sessionLength	= 900	// 30 mins
)

var tpl *template.Template

var (
	clientId, secretKey, key string
)

func init() {
	e := godotenv.Load()
	if e != nil {
		log.Fatal("Error loading .env file")
	}
	clientId = os.Getenv("SPOTIFY_ID")
	secretKey = os.Getenv("SPOTIFY_SECRET")
	key = os.Getenv("ENCRYPTION_KEY")
	spotifyAuth.SetAuthInfo(clientId, secretKey)
	tpl = template.Must(template.ParseGlob("templates/*"))
	//sessionsCleaned := time.Now()
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", playerHandler)
	mux.HandleFunc("/authenticate", initAuth)
	mux.HandleFunc("/callback", completeAuth)

	mux.Handle("/favicon.ico", http.NotFoundHandler())

	fmt.Printf("Listening on port %d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), authMiddleware(mux))
}

func playerHandler(w http.ResponseWriter, r *http.Request) {
	var playerState *spotify.PlayerState

	client, e := getClient(w, r)
	if e != nil {
		return
	}

	// use the client to make calls that require authorization
	user, e := client.CurrentUser()
	if e != nil {
		log.Fatalf("Getting user: %s", e.Error())
	}
	fmt.Println("You are logged in as:", user.ID)

	playerState, e = client.PlayerState()

	currPlaying := playerState.CurrentlyPlaying
	artist := currPlaying.Item.SimpleTrack.Artists[0].Name
	title := currPlaying.Item.SimpleTrack.Name
	fmt.Printf("Artist: %s, Title: %s\n", artist, title)

	lyric := getLyrics(artist, title)
	fmt.Fprintf(w, lyric)

	fmt.Printf("Found your %s (%s)\n", playerState.Device.Type, playerState.Device.Name)
}

func getLyrics(artist, title string) string {
	l := lyrics.New()
	lyric, err := l.Search(artist, title)
	if err != nil {
		fmt.Printf("Can't fetch lyrics: %s\n", err.Error())
		return "Not found"
	}
	return lyric
}