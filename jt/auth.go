package jt

import (
	"./json"
	"code.google.com/p/goauth2/oauth"
	"crypto/tls"
	jzon "encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/gorilla/sessions"
	"io/ioutil"
	"log"
	"net/http"
	// "os"
)

type Auth struct {
	ClientID     string
	ClientSecret string
	Scope        string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	RequestURL   string
}

var authcfg *oauth.Config
var store = sessions.NewCookieStore([]byte("fix-this-later"))

func loadAuthConfig() {
	data, err := ioutil.ReadFile("auth.json")
	if err != nil {
		panic(err)
	}

	obj := json.LoadJson(data)

	if !obj.Has("web") {
		panic("Invalid auth payload")
	}

	web := obj.Get("web")
	authcfg = &oauth.Config{
		ClientId:     web.VStr("client_id"),
		ClientSecret: web.VStr("client_secret"),
		RedirectURL:  web.Value("redirect_uris").([]interface{})[0].(string),
		Scope:        "https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email",
		AuthURL:      web.VStr("auth_uri"),
		TokenURL:     web.VStr("token_uri"),
		TokenCache:   oauth.CacheFile("cache.json"),
	}
}

func setup_auth() {
	loadAuthConfig()
}

// Start the authorization process
func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, authcfg.AuthCodeURL(""), http.StatusFound)
}

func FindUser(email string) json.Object {
	users := DB.Use("users")

	queryStr := fmt.Sprintf(`[{"eq": "%s", "in": ["email"]}]`, email)
	var query interface{}
	jzon.Unmarshal([]byte(queryStr), &query)

	queryResult := make(map[uint64]struct{})
	if err := db.EvalQuery(query, users, &queryResult); err != nil {
		return nil
	}

	if len(queryResult) > 0 {
		for id := range queryResult {
			return GetUser(id)
		}
	}

	return nil
}

func GetUser(id uint64) json.Object {
	users := DB.Use("users")
	var data interface{}

	users.Read(id, &data)
	if data == nil {
		return nil
	}

	obj := json.Loader(data.(map[string]interface{}))
	obj.Set("id", id)
	return obj
}

// Function that handles the callback from the Google server
func handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	users := DB.Use("users")
	var user json.Object
	var id uint64

	//Get the code from the response
	code := r.FormValue("code")

	t := &oauth.Transport{Config: authcfg}

	// Exchange the received code for a token
	tok, _ := t.Exchange(code)

	tokenCache := oauth.CacheFile("./request.token")

	err := tokenCache.PutToken(tok)
	if err != nil {
		log.Fatal("Cache write:", err)
	}
	log.Printf("Token is cached in %v\n", tokenCache)

	// Skip TLS Verify
	t.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Make the request.
	req, err := t.Client().Get("https://www.googleapis.com/oauth2/v1/userinfo?alt=json")
	if err != nil {
		log.Fatal("Request Error:", err)
	}
	defer req.Body.Close()

	body, _ := ioutil.ReadAll(req.Body)
	log.Printf("Body: %s", body)

	// TODO cleanuup
	data := json.LoadJson(body)
	if !data.Has("hd") {
		return
	}

	hd := data.VStr("hd")
	found := false
	for _, x := range CONFIG.Value("tlds").([]interface{}) {
		if x.(string) == hd {
			found = true
			break
		}
	}

	if !found {
		return
	}

	user = FindUser(data.VStr("email"))
	if user == nil {
		data.Del("id")
		id, _ = users.Insert(data)
		log.Printf("New user logged in w/ id %s", id)
	} else {
		id = user.Value("id").(uint64)
		user.Set("picture", data.VStr("picture"))
		user.Set("given_name", data.VStr("given_name"))
		users.Update(id, user)
		log.Printf("User relogged in w/ id %s", id)
	}

	session, _ := store.Get(r, "justtalk")
	session.Values["id"] = id
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusFound)
}
