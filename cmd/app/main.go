package app

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/zmb3/spotify"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
)

const redirectURI = "http://localhost:8080/callback"

var html = `
<br/>
<a href="/player/play">Play</a><br/>
<a href="/player/pause">Pause</a><br/>
<a href="/player/next">Next track</a><br/>
<a href="/player/previous">Previous Track</a><br/>
<a href="/player/shuffle">Shuffle</a><br/>
`

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
	key = os.Getenv("KEY")
	spotifyAuth.SetAuthInfo(clientId, secretKey)
	tpl = template.Must(template.ParseGlob("templates/*"))
	//sessionsCleaned := time.Now()
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/authenticate", initAuth)
	mux.HandleFunc("/callback", completeAuth)
	mux.HandleFunc("/player", playerHandler)

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

	if e != nil {
		log.Fatalf("Getting player state: %s", e.Error())
	}
	fmt.Printf("Found your %s (%s)\n", playerState.Device.Type, playerState.Device.Name)
	action := strings.TrimPrefix(r.URL.Path, "/player/")
	fmt.Println("Got request for:", action)
	var err error
	switch action {
	case "play":
		err = client.Play()
	case "pause":
		err = client.Pause()
	case "next":
		err = client.Next()
	case "previous":
		err = client.Previous()
	case "shuffle":
		playerState.ShuffleState = !playerState.ShuffleState
		err = client.Shuffle(playerState.ShuffleState)
	}
	if err != nil {
		log.Print(err)
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

func index(w http.ResponseWriter, r *http.Request) {
	log.Println("Got request for:", r.URL.String())
	c, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "Cookie not found\n", http.StatusInternalServerError)
		return
	}

	s, ok := sessions[c.Value]
	if !ok {
		http.Error(w, "Client not found\n", http.StatusInternalServerError)
		return
	}

	_ = s
	fmt.Println("End of index")
	//tpl.ExecuteTemplate(w, "index.gohtml", c.Value, s.client)
	fmt.Fprintf(w, "Session Id: %s", c.Value)
}