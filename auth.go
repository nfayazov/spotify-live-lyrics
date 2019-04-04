package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/satori/go.uuid"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
	"net/http"
	"spotify-live-lyricist/pkg/encrypt"
	"time"
)

type session struct {
	Token        []byte
	LastActivity time.Time
}

const (
	sessionLength			= 900	// 30 mins
	sessionCleanupInterval 	= 15	// 10 mins
)

var (
	states      = make(map[string]string)	// sID, oauth state
	sessions    = make(map[string]session)	// sessionId, session
	lastSessionsCleaned time.Time
)

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func (w http.ResponseWriter, req *http.Request) {
		// first things first
		//go cleanSessions()

		if req.URL.Path != "/authenticate" && req.URL.Path != "/callback" {
			c, err := req.Cookie("session")
			if err != nil {
				http.Redirect(w, req, "/authenticate", http.StatusSeeOther)
				return
			}

			//s, ok := sessions[c.Value]
			s, err := getSessionFromRedis(c.Value)
			if err == nil {
				s.LastActivity = time.Now()
				err := setSession(c.Value, *s)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				//sessions[c.Value] = s
			} else if err == redis.ErrNil {
				http.Redirect(w, req, "/authenticate", http.StatusSeeOther)
				return
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// refresh session
			c.MaxAge = sessionLength
			http.SetCookie(w, c)
		}

		next.ServeHTTP(w, req)
	})
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

	url := spotifyAuth.AuthURL(state.String())
	states[sID.String()] = state.String()

	//fmt.Println("Please log in to Spotify by visiting the following page in your browser:", url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := checkStateAndGetToken(w, r)
	if err != nil {
		fmt.Print(err)
		return
	}

	encToken, err := marshalAndEncryptToken(w, tok)
	if err != nil {
		http.Error(w, fmt.Sprintf("Token Error: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	_, err = createSession(w, encToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	fmt.Println("Successfully authenticated")
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// Looks for sID cookie (which represents oauth state), checks if it matched, get token and deletes state
func checkStateAndGetToken(w http.ResponseWriter, r *http.Request) (*oauth2.Token, error) {
	sID, err := r.Cookie("sID")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}

	// delete cookie from client
	sID.MaxAge = -1
	http.SetCookie(w, sID)

	state, ok := states[sID.Value]
	if !ok {
		http.Error(w, "OAuth state not found", http.StatusInternalServerError)
		return nil, errors.New("OAuth state not found")
	}

	if st := r.FormValue("state"); st != state {
		http.Error(w, fmt.Sprintf("State mismatch: %s != %s\n", st, state), http.StatusNotFound)
		return nil, errors.New("State mismatch\n")
	}

	tok, err := spotifyAuth.Token(state, r)

	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		return nil, err
	}

	// state no longer needed
	delete(states, sID.Value)

	return tok, nil
}

func marshalAndEncryptToken(w http.ResponseWriter, tok *oauth2.Token) ([]byte, error) {
	bs, err := json.Marshal(*tok)
	if err != nil {
		return nil, err
	}

	enc := encrypt.Encrypt(key, string(bs))
	return enc, nil
}

// Passing encrypted access token forces binding between local sessions and oAuth sessions.
func createSession(w http.ResponseWriter, encToken []byte) (*session, error) {
	// create session
	sID, _ := uuid.NewV4()
	c := &http.Cookie{
		Name:  "session",
		Value: sID.String(),
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	// TODO: encrypt sessionId
	//encSessionId := encrypt.Encrypt(key, )

	s := session{encToken,time.Now()}

	// Save to Redis
	fmt.Printf("SessionID: %s\n", sID.String())
	err := setSession(c.Value, s)
	if err != nil {
		return nil, err
	}
	//sessions[c.Value] = s

	return &s, nil
}

// getClient gets token from session and exchanges for client
func getClient(w http.ResponseWriter, req *http.Request) (*spotify.Client, error) {
	token := &oauth2.Token{}

	// get session from cookie
	sesh, err := getSession(w, req)
	if err != nil {
		return nil, err
	}

	// get token from session
	jsonToken := encrypt.Decrypt(key, sesh.Token)
	err = json.Unmarshal([]byte(jsonToken), token)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling token: %s", err.Error()), http.StatusInternalServerError)
		return nil, err
	}

	client := spotifyAuth.NewClient(token)

	return &client, nil
}

func getSession(w http.ResponseWriter, req *http.Request) (*session, error) {
	c, err := req.Cookie("session")
	if err != nil {
		return nil, err
	}

	sesh, err := getSessionFromRedis(c.Value)
	if err != nil {
		return nil, err
	}

	// TODO: Check if session expired

	/*s, ok := sessions[c.Value]
	if !ok {
		http.Error(w, "Session not found\n", http.StatusForbidden)
		return nil, err
	}*/

	return sesh, nil
}

/*func cleanSessions() {
	if time.Now().Sub(lastSessionsCleaned) < (sessionCleanupInterval * time.Second) {
		return
	}

	var s session

	keys, err := redis.Strings(conn.Do("KEYS", "session:*"))
	if err != nil {
		fmt.Printf("Error getting sessions from Redis while cleaning\n")
		return
	}

	for k, _ := range keys {

	}

	r, err := redis.Scan(values, &s)
	_ = r

	//for k, s := range allSessions {
	//	if time.Now().Sub(s.LastActivity) > sessionLength*time.Second {
	//		delete(sessions, k)
	//	}
	//}

	lastSessionsCleaned = time.Now()
}*/

func logout(w http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("session")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	c.MaxAge = -1
	http.SetCookie(w, c)

	delete(sessions, c.Value)
	fmt.Fprint(w, "Successfully logged out")
}