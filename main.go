package main

import (
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"spotify-live-lyricist/pkg/lyricTreeSet"
	"strconv"
	"time"

	"github.com/rhnvrm/lyric-api-go"
	"github.com/zmb3/spotify"
	"html/template"
	"net/http"
	"os"
	"sync"
)

const cacheLimit = 300

type cache struct {
	lSet		*lyricTreeSet.LyricsSet
	mutex		sync.Mutex
}

var (
	port = 8080
	tpl *template.Template
	lyricCache *cache
	clientId, secretKey, key, redirectURI string
	spotifyAuth spotify.Authenticator
)

func init() {
	// get env variables
	p, _ := strconv.Atoi(os.Getenv("PORT"))
	if p != 0 {
		port = p
	}

	if os.Getenv("PRODUCTION") != "true" {
		redirectURI = "http://localhost:8080/callback"
		e := godotenv.Load()
		if e != nil {
			log.Fatal("Error loading .env file")
		}
	} else {
		redirectURI = "https://lyrics-spotify.herokuapp.com/callback"
	}

	clientId = os.Getenv("SPOTIFY_ID")
	secretKey = os.Getenv("SPOTIFY_SECRET")
	key = os.Getenv("ENCRYPTION_KEY")

	spotifyAuth = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying, spotify.ScopeUserReadPlaybackState, spotify.ScopeUserModifyPlaybackState)
	spotifyAuth.SetAuthInfo(clientId, secretKey)
	tpl = template.Must(template.ParseGlob("templates/*"))

	lyricCache = &cache{
		lSet: lyricTreeSet.New(cacheLimit),
	}

	lastSessionsCleaned = time.Now()
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
	client, e := getClient(w, r)
	if e != nil {
		return
	}

	artist, title, err := getSpotifyTrack(client, w)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	lyric := getCachedLyrics(artist, title)
	fmt.Fprintf(w, lyric)

}

func getSpotifyTrack(client *spotify.Client, w http.ResponseWriter) (string, string, error) {
	var playerState *spotify.PlayerState

	user, e := client.CurrentUser()
	if e != nil {
		fmt.Printf("Getting user: %s", e.Error())
		return "", "", e
	}
	fmt.Println("You are logged in as:", user.ID)

	playerState, e = client.PlayerState()

	currPlaying := playerState.CurrentlyPlaying
	artist := currPlaying.Item.SimpleTrack.Artists[0].Name
	title := currPlaying.Item.SimpleTrack.Name
	fmt.Printf("Artist: %s, Title: %s\n", artist, title)
	fmt.Printf("Found your %s (%s)\n", playerState.Device.Type, playerState.Device.Name)

	return artist, title, nil
}

func getCachedLyrics(artist, title string) string {
	// look if in the cache, if yes - return
	val, ok := lyricCache.lSet.Get(artist, title)
	if ok {
		fmt.Println("Getting from cache")
		return val
	}

	// if not, then call get lyrics
	lyrics, err := getLyrics(artist, title)
	if err != nil {
		return lyrics // return "Not found"
	}

	// add new lyric to cache
	fmt.Println("Updating cache")
	lyricCache.mutex.Lock()
	defer lyricCache.mutex.Unlock()
	lyricCache.lSet.Put(artist, title, lyrics)

	return lyrics
}

func getLyrics(artist, title string) (string, error) {
	l := lyrics.New()
	lyric, err := l.Search(artist, title)
	if err != nil {
		fmt.Printf("Can't fetch lyrics: %s\n", err.Error())
		return "Not found", errors.New("not found")
	}
	return lyric, nil
}

