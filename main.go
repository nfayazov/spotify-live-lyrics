package main

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/joho/godotenv"
	"log"
	"spotify-live-lyricist/pkg/lyricTreeSet"
	"strconv"
	"strings"
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
	conn redis.Conn
)

type Result struct {
	Username				string
	DeviceType, DeviceName	string
	Artist, Title 			string
	Text					template.HTML
}

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
		hostUrl := os.Getenv("HOST_URL")
		redirectURI = hostUrl + "/callback"
	}

	clientId = os.Getenv("SPOTIFY_ID")
	secretKey = os.Getenv("SPOTIFY_SECRET")
	key = os.Getenv("ENCRYPTION_KEY")

	// Configure Spotify
	spotifyAuth = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying, spotify.ScopeUserReadPlaybackState, spotify.ScopeUserModifyPlaybackState)
	spotifyAuth.SetAuthInfo(clientId, secretKey)
	tpl = template.Must(template.ParseGlob("templates/*"))

	lyricCache = &cache{
		lSet: lyricTreeSet.New(cacheLimit),
	}

	lastSessionsCleaned = time.Now()

	// Configure Redis
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	address := redisHost+":"+redisPort // "localhost:6379"
	if address == ":" { // dev
		address = ":6379"
	}
	pool := newPool(address)
	conn = pool.Get()

	tpl = template.Must(template.ParseGlob("templates/*.gohtml"))
}

func main() {

	mux := http.NewServeMux()
	mux.HandleFunc("/", playerHandler)
	mux.HandleFunc("/authenticate", initAuth)
	mux.HandleFunc("/callback", completeAuth)
	mux.HandleFunc("/logout", logout)
	mux.Handle("/favicon.ico", http.NotFoundHandler())

	fmt.Printf("Listening on port %d\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), authMiddleware(mux))
}

func playerHandler(w http.ResponseWriter, r *http.Request) {
	client, e := getClient(w, r)
	if e != nil {
		return
	}

	result, err := getSpotifyTrack(client, w)
	if err != nil {
		http.Error(w, fmt.Sprintf(err.Error()), http.StatusInternalServerError)
		return
	}

	lyrics := getCachedLyrics(result.Artist, result.Title)
	lyrics = strings.Replace(lyrics, "\n", "<br>", -1) // replace all newlines with proper html tag
	result.Text = template.HTML(lyrics)
	err = tpl.ExecuteTemplate(w, "index.gohtml", result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

func getSpotifyTrack(client *spotify.Client, w http.ResponseWriter) (*Result, error) {
	var playerState *spotify.PlayerState
	result := &Result{}

	user, e := client.CurrentUser()
	if e != nil {
		fmt.Printf("Getting user: %s", e.Error())
		return nil, e
	}
	result.Username = user.ID

	playerState, e = client.PlayerState()
	currPlaying := playerState.CurrentlyPlaying
	if currPlaying.Playing == true && currPlaying.Item != nil {
		result.Artist = currPlaying.Item.SimpleTrack.Artists[0].Name
		result.Title = currPlaying.Item.SimpleTrack.Name
		result.DeviceType = playerState.Device.Type
		result.DeviceName = playerState.Device.Name
		return result, nil
	} else {
		return nil, errors.New("Spotify Track Not Found\n")
	}

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
		return "Lyrics not found :(", errors.New("not found")
	}
	return lyric, nil
}

