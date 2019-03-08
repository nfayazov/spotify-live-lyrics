package auth

import (
	"encoding/json"
	"encrypt"
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
	"net/http"
	"time"
)

type session struct {
	token			[]byte
	lastActivity	time.Time
}

var (
	states      = make(map[string]string)	// sID, oauth state
	sessions    = make(map[string]session)	// sessionId, session
	spotifyAuth = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadCurrentlyPlaying, spotify.ScopeUserReadPlaybackState, spotify.ScopeUserModifyPlaybackState)
)

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

	createSession(w, encToken)

	fmt.Println("Successfully authenticated")
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// Looks for sID cookie (which represents oauth state), checks if it matched, get token and deletes state
func checkStateAndGetToken(w http.ResponseWriter, r *http.Request) (*oauth2.Token, error) {
	sID,err := r.Cookie("sID")
	if err != nil {
		http.Error(w, "OAuth cookie not found", http.StatusInternalServerError)
		return nil, err
	}

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
func createSession(w http.ResponseWriter, encToken []byte) {
	// create session
	sID, _ := uuid.NewV4()
	c := &http.Cookie{
		Name:  "session",
		Value: sID.String(),
	}
	c.MaxAge = sessionLength
	http.SetCookie(w, c)

	sessions[c.Value] = session{encToken,time.Now()}
}

func getClient(w http.ResponseWriter, req *http.Request) (*spotify.Client, error) {
	var client *spotify.Client
	token := &oauth2.Token{}

	// get session from cookie
	sesh, err := getSession(w, req)
	if err != nil {
		return nil, err
	}

	// get token from session
	jsonToken := encrypt.Decrypt(key, sesh.token)
	err = json.Unmarshal([]byte(jsonToken), token)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling client: %s", err.Error()), http.StatusInternalServerError)
		return nil, err
	}

	*client = spotifyAuth.NewClient(token)

	return client, nil
}

func getSession(w http.ResponseWriter, req *http.Request) (session, error) {
	c, err := req.Cookie("session")
	if err != nil {
		http.Error(w, "Session not found\n", http.StatusForbidden)
		return session{}, err
	}

	s, ok := sessions[c.Value]
	if !ok {
		http.Error(w, "Session not found\n", http.StatusForbidden)
		return session{}, err
	}

	return s, nil
}
