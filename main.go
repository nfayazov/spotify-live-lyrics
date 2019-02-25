// This example demonstrates how to authenticate with Spotify using the authorization code flow.
// In order to run this example yourself, you'll need to:
//
//  1. Register an application at: https://developer.spotify.com/my-applications/
//       - Use "http://localhost:8080/callback" as the redirect URI
//  2. Set the SPOTIFY_ID environment variable to the client ID you got in step 1.
//  3. Set the SPOTIFY_SECRET environment variable to the client secret from step 1.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/satori/go.uuid"
	"github.com/zmb3/spotify"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"os"
	"time"
)

// redirectURI is the OAuth redirect URI for the application.
// You must register an application at Spotify's developer portal
// and enter this value.
const redirectURI = "http://localhost:8080/callback"

var html = `
<br/>
<a href="/player/play">Play</a><br/>
<a href="/player/pause">Pause</a><br/>
<a href="/player/next">Next track</a><br/>
<a href="/player/previous">Previous Track</a><br/>
<a href="/player/shuffle">Shuffle</a><br/>
`

type session struct {
	client			[]byte
	lastActivity	time.Time
}

var (
	clientId, secretKey string
	auth  			= spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying, spotify.ScopeUserReadPlaybackState, spotify.ScopeUserModifyPlaybackState)
)

var (
	states			= make(map[string]string)	// sID, oauth state
	sessions		= make(map[string]session)	// sessionId, session
	sessionLength	= 900	// 30 mins
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	clientId = os.Getenv("SPOTIFY_ID")
	secretKey := os.Getenv("SPOTIFY_SECRET")
	auth.SetAuthInfo(clientId, secretKey)
	//sessionsCleaned := time.Now()
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/authenticate" && req.URL.Path != "/callback" {
			c, err := req.Cookie("session")
			if err != nil {
				http.Redirect(w, req, "/authenticate", http.StatusSeeOther)
				return
			}

			s, ok := sessions[c.Value]
			if ok {
				s.lastActivity = time.Now()
				sessions[c.Value] = s
			} else {
				http.Redirect(w, req, "/authenticate", http.StatusSeeOther)
				return
			}

			// refresh session
			c.MaxAge = sessionLength
			http.SetCookie(w, c)
		}

		next.ServeHTTP(w, req)
	})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", completeAuth)
	/*http.HandleFunc("/player/", func(w http.ResponseWriter, r *http.Request) {
		var client *spotify.Client
		var playerState *spotify.PlayerState


		// use the token to get an authenticated client
		tmp := auth.NewClient(tok)
		client = &tmp

		// use the client to make calls that require authorization
		user, err := client.CurrentUser()
		if err != nil {
			log.Fatalf("Getting user: %s", err.Error())
		}
		fmt.Println("You are logged in as:", user.ID)

		playerState, err = client.PlayerState()

		if err != nil {
			log.Fatalf("Getting player state: %s", err.Error())
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
	})*/

	mux.HandleFunc("/authenticate", initAuth)

	mux.Handle("/favicon.ico", http.NotFoundHandler())
	mux.HandleFunc("/", index)

	http.ListenAndServe(":8080", authMiddleware(mux))
}

func initAuth(w http.ResponseWriter, r *http.Request) {
	// create cookie for oauth state
	sID, _ := uuid.NewV4()
	state, _ := uuid.NewV4()
	c := &http.Cookie{
		Name:  "sID",
		Value: sID.String(),
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	url := auth.AuthURL(state.String())
	states[sID.String()] = state.String()
	//fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	spotifyClient, err := getClientFromToken(w, r)
	if err != nil {
		fmt.Print(err)
		return
	}

	encoded, err := json.Marshal(spotifyClient)
	if err != nil {
		http.Error(w, "", 500)
		fmt.Println("Marshalling error")
	}

	encClient, err := bcrypt.GenerateFromPassword([]byte(encoded), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "", 500)
		fmt.Println("Encryption error")
	}

	createSession(w, encClient)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func getClientFromToken(w http.ResponseWriter, r *http.Request) (spotify.Client, error) {
	sID,err := r.Cookie("sID")
	if err != nil {
		http.Error(w, "OAuth cookie not found", http.StatusInternalServerError)
		return spotify.Client{}, err
	}

	state, ok := states[sID.Value]
	if !ok {
		http.Error(w, "OAuth state not found", http.StatusInternalServerError)
		return spotify.Client{}, errors.New("OAuth state not found")
	}

	tok, err := auth.Token(state, r)

	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		return spotify.Client{}, err
	}

	if st := r.FormValue("state"); st != state {
		http.Error(w, fmt.Sprintf("State mismatch: %s != %s\n", st, state), http.StatusNotFound)
		return spotify.Client{}, errors.New("State mismatch\n")
	}

	fmt.Printf("Access Token: %s\n", tok.AccessToken)
	return auth.NewClient(tok), nil
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

	fmt.Fprintf(w, "Session Id: %s, Encrypted Client: %s", c.Value, s.client)
	fmt.Print(r.Header)
	defer r.Body.Close()
}

// Passing encrypted spotify.Client forces binding between local sessions and oAuth sessions.
func createSession(w http.ResponseWriter, client []byte) {
	// create session
	sID, _ := uuid.NewV4()
	c := &http.Cookie{
		Name:  "session",
		Value: sID.String(),
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	sessions[c.Value] = session{client,time.Now()}
}