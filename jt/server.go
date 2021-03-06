package jt

import (
	"./json"
	"crypto/md5"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/gorilla/websocket"
	"github.com/kennygrant/sanitize"
	"github.com/russross/blackfriday"
	"github.com/vmihailenco/redis/v2"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"
)

type CmdFunc func(*Connection, *Channel, json.Object, []string)

var CONNS map[string]*Connection = make(map[string]*Connection, 0)
var CHANS map[string]*Channel = make(map[string]*Channel, 0)
var CMDS map[string]CmdFunc = make(map[string]CmdFunc, 0)
var DB *db.DB
var RED *redis.Client
var OUTERP, _ = regexp.Compile("<p>(.+)</p>")
var URLREG, _ = regexp.Compile(`/((([A-Za-z]{3,9}:(?:\/\/)?)(?:[-;:&=\+\$,\w]+@)?[A-Za-z0-9.-]+|(?:www.|[-;:&=\+\$,\w]+@)[A-Za-z0-9.-]+)((?:\/[\+~%\/.\w-_]*)?\??(?:[-\+=&;%@.\w_]*)#?(?:[\w]*))?)/`)
var CONFIG json.Object

func NotOuterP(s string) string {
	val := OUTERP.FindSubmatch([]byte(s))
	if len(val) > 1 {
		return string(val[1])
	}
	return s
}

func IsUrlImage(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}

	if IsImage(resp.Header.Get("Content-Type")) {
		return true
	}

	return false
}

func Bind(f CmdFunc, v ...string) {
	for _, item := range v {
		CMDS[item] = f
	}
}

func IsImage(s string) bool {
	switch s {
	case "image/gif":
		return true
	case "image/jpeg":
		return true
	case "image/png":
		return true
	}
	return false
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)

	if user == nil {
		http.Error(w, "Yeah right...", 400)
		return
	}

	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		log.Printf("handleWebSocket err: %v", err)
		http.Error(w, "Error opening websocket!", 500)
	}

	c := &Connection{
		Buffer: make(chan []byte, 256),
		ws:     ws,
	}
	c.LoadUser(user)
	go c.ReadLoop()
	c.WriteLoop()
}

func getAvatarHash(name string) string {
	h := md5.New()
	io.WriteString(h, name)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func getAvatarUrl(name string) string {
	return fmt.Sprintf("http://www.gravatar.com/avatar/%s?s=64&d=identicon&r=X", getAvatarHash(name))
}

func hook_exit() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			log.Printf("Goodbye %s!", sig)
			DB.Close()
			RED.Close()
			os.Exit(0)
		}
	}()
}

func setup_db() {
	rand.Seed(time.Now().UTC().UnixNano())
	new_db, err := db.OpenDB("database")
	if err != nil {
		panic(err)
	}
	DB = new_db

	DB.Create("channels", 2)
	chans := DB.Use("channels")
	chans.Index([]string{"name"})

	DB.Create("users", 2)
	users := DB.Use("users")
	users.Index([]string{"email"})
}

func setup_redis() {
	if !CONFIG.Has("redis") {
		return
	}

	RED = redis.NewTCPClient(&redis.Options{
		Addr: CONFIG.VStr("redis"),
	})
}

func web_send_to(w http.ResponseWriter, r *http.Request) {
	asstr, _ := ioutil.ReadAll(r.Body)
	log.Printf("Data: `%s`", asstr)
	obj := json.LoadJson(asstr)
	if !obj.Has("channel") || !obj.Has("msg") {
		http.Error(w, "Invalid Payload", 400)
		return
	}

	channel, check := CHANS[obj.VStr("channel")]
	if !check {
		http.Error(w, "Invald Channel", 400)
		return
	}

	data := json.Object{
		"type":   "action",
		"action": obj.VStr("msg"),
		"raw":    obj.Value("raw").(bool),
		"icon":   obj.VStr("icon"),
		"dest":   channel.Name,
	}

	log.Printf("Sending message from API")
	if obj.Has("user") {
		user, check := CONNS[obj.VStr("user")]
		if !check {
			http.Error(w, "Invalid User", 400)
			return
		}
		user.Send(data)
	} else {
		channel.SendRaw(data)
	}
}

func web_upload(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Yeah right!", 400)
		return
	}

	err := r.ParseMultipartForm(100000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	channel_name := r.URL.Query().Get("channel")
	channel, check := CHANS[channel_name]
	if !check {
		http.Error(w, "Invalid Channel!", 400)
		return
	}

	m := r.MultipartForm

	files := m.File["file"]
	results := make([]string, 0)
	for i, _ := range files {
		key := fmt.Sprintf("file-%v", rand.Int63())
		file, err := files[i].Open()
		defer file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		raw, _ := ioutil.ReadAll(file)
		if len(raw) > 5242880 {
			http.Error(w, "File is too big!", 400)
			return
		}

		RED.SetEx(key, time.Minute*60, string(raw))
		RED.SetEx(key+"-type", time.Minute*60, files[i].Header.Get("Content-Type"))
		log.Printf("Uploaded file %s", key)
		log.Printf("%s", files[i].Header)

		if IsImage(files[i].Header.Get("Content-Type")) {
			u, c := CONNS[user.VStr("email")]
			if !c {
				http.Error(w, "Invalid User!", 400)
				return
			}

			u.SendImage(channel, fmt.Sprintf("/api/file?key=%s", key))
		} else {
			results = append(results, fmt.Sprintf(`<a href="/api/file?key=%s">%s</a>`, key, files[i].Filename))
		}
	}

	if len(results) > 0 {
		msg := fmt.Sprintf("%s uploaded %v files: %s", user.VStr("given_name"), len(results), strings.Join(results, ", "))
		channel.SendRaw(json.Object{
			"type":   "action",
			"action": msg,
			"raw":    true,
			"icon":   "upload",
			"dest":   channel_name,
		})
	}
}

func DecrementLimit() {
	for {
		for _, user := range CONNS {
			if user.Limit > 0 {
				user.Limit -= 1
			}
		}
		time.Sleep(time.Second * 3)
	}
}

func GetUserFromRequest(req *http.Request) json.Object {
	session, _ := store.Get(req, "justtalk")
	id, has := session.Values["id"]
	if !has {
		return nil
	}

	user := GetUser(id.(uint64))
	if !user.Has("email") {
		return nil
	}

	return user
}

func web_user(rw http.ResponseWriter, req *http.Request) {
	user := GetUserFromRequest(req)
	if user == nil {
		fmt.Fprint(rw, `{"success": false}`)
		return
	}

	user.Set("success", true)

	data, _ := user.Dump()
	rw.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(rw, "%s", data)
	return
}

func handleLogout(rw http.ResponseWriter, req *http.Request) {
	session, _ := store.Get(req, "justtalk")
	delete(session.Values, "id")
	session.Save(req, rw)
	http.Redirect(rw, req, "/", http.StatusMovedPermanently)
}

func web_file(rw http.ResponseWriter, req *http.Request) {
	user := GetUserFromRequest(req)
	if user == nil {
		http.Error(rw, "Yeah right!", 400)
		return
	}

	key := req.URL.Query().Get("key")
	log.Printf("Getting file: `%s`", key)
	if !RED.Exists(key).Val() {
		http.Error(rw, "That file does not exist!", 404)
		return
	}

	rw.Header().Set("Content-Type", RED.Get(key+"-type").Val())
	log.Printf("Size: %s", len(RED.Get(key).Val()))
	fmt.Fprintf(rw, "%s", RED.Get(key).Val())
}

func LoadConfig() {
	data, err := ioutil.ReadFile("config.js")
	if err != nil {
		panic("No config! Please read example_config.js for more information.")
	}
	CONFIG = json.LoadJson(data)
}

func Run() {
	// Loads and validates config
	LoadConfig()

	// Setup some common things
	setup_db()
	setup_redis()
	setup_auth()

	// Grab Ctrl+C signals
	hook_exit()

	// These are the HTTP routes
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/socket", handleWebSocket)
	http.HandleFunc("/api/send", web_send_to)
	http.HandleFunc("/api/upload", web_upload)
	http.HandleFunc("/api/user", web_user)
	http.HandleFunc("/api/file", web_file)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/authorize", handleAuthorize)
	http.HandleFunc("/oauth2callback", handleOAuth2Callback)

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /img <url>"})
		}

		if IsUrlImage(args[1]) {
			u.SendImage(c, args[1])
		} else {
			u.SendS(ChatError{Msg: "Error: Not a valid image!"})
		}

	}, "img", "i")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /join <channel>"})
			return
		}
		chan_name := strings.ToLower(args[1])

		if _, chan_exists := CHANS[chan_name]; !chan_exists {
			channel := NewChannel(chan_name, chan_name, "", getAvatarUrl(chan_name), false)
			CHANS[chan_name] = channel
		}

		// We cannot join private message lobbies
		if CHANS[chan_name].PM {
			u.SendS(ChatError{Msg: "Invalid Channel!"})
			return
		}

		CHANS[chan_name].Join(u)
	}, "join", "j")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		if c.Name == "lobby" {
			u.SendS(ChatError{
				Msg: "You cannot leave the lobby!",
			})
			return
		}

		if c.PM {
			c.Delete()
			return
		}

		c.Quit(u, "%s has left the channel")
	}, "quit", "q")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		if c.Name == "lobby" {
			u.Send(json.Object{
				"type": "error",
				"msg":  "You cannot delete the lobby!",
			})
			return
		}

		if c.PM {
			u.SendS(ChatError{
				Msg: "That action is not availbile for Private Messages.",
			})
			return
		}

		c.Delete()
	}, "delete")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /action <action>"})
			return
		}

		c.SendRaw(json.Object{
			"type":   "action",
			"action": strings.Join(args[1:], " "),
			"raw":    false,
			"icon":   "exclamation-circle",
			"dest":   c.Name,
			"color":  "red",
		})
	}, "action", "act", "a")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		resp := make(json.Object)
		if len(args) < 3 {
			u.SendS(ChatError{Msg: "Usage: /cset <option> <value>"})
			return
		}

		resp.Set("type", "updatechannel")
		resp.Set("name", c.Name)

		if c.Name == "lobby" {
			u.SendS(ChatError{Msg: "Cannot edit the lobby!"})
			return
		}

		if c.PM {
			u.SendS(ChatError{
				Msg: "That action is not availbile for Private Messages.",
			})
			return
		}

		if args[1] == "topic" {
			c.Topic = NotOuterP(string(blackfriday.MarkdownCommon([]byte(sanitize.HTML(strings.Join(args[2:], " "))))))
			resp.Set("k", "topic")
			resp.Set("v", c.Topic)
			resp.Set("a", fmt.Sprintf("%s has changed the channel topic", u.Name))
		} else if args[1] == "image" {
			c.Image = sanitize.HTML(args[2])
			resp.Set("k", "image")
			resp.Set("v", c.Image)
			resp.Set("a", fmt.Sprintf("%s has changed the channel icon", u.Name))
		} else if args[1] == "title" {
			c.Title = sanitize.HTML(strings.Join(args[2:], " "))
			if len(c.Title) > 25 {
				c.Title = c.Title[:25]
			}
			resp.Set("k", "title")
			resp.Set("v", c.Title)
			resp.Set("a", fmt.Sprintf("%s has changed the channel title", u.Name))
		} else {
			u.SendS(ChatError{Msg: "Channel Set Values: topic, image, title"})
			return
		}

		c.SendRaw(resp)
		c.Save()

	}, "cset", "c")

	Bind(func(u *Connection, c *Channel, o json.Object, args []string) {
		//resp := make(json.Object)
		if len(args) < 2 {
			u.SendS(ChatError{
				Msg: fmt.Sprintf("Usage: /%s <user>", args[0]),
			})
			return
		}

		name := strings.ToLower(args[1])
		user, uch := CONNS[name]
		if !uch {
			u.SendS(ChatError{Msg: fmt.Sprintf("No user `%s`!", name)})
			return
		}

		if user == u {
			u.SendS(ChatError{Msg: "You cannot message yourself!"})
			return
		}

		chan_name := fmt.Sprintf("!PM!%v", rand.Int31())
		pm_chan := NewChannel(chan_name, "", "", "", true)

		// Add this as a valid channel
		CHANS[chan_name] = pm_chan

		// Add the users to the PM, don't send JOIN packets
		pm_chan.BuildJoin(u)
		pm_chan.BuildJoin(user)

		// Now send packets
		u.Send(pm_chan.ToJson())
		user.Send(pm_chan.ToJson())

		log.Printf("Yeaaaah!")
	}, "pm", "msg")

	// Default load the lobby
	CHANS["lobby"] = NewChannel("lobby", "The Lobby", "Sit down and have a cup of tea", "https://lh5.ggpht.com/LkzyZWEvMWSym5etth9H3a2vMCxUZFNW99seYYF6XPKIGNvY3m1YzTe0QCDMQB9G0QM=w300", false)

	// Rate limiting
	go DecrementLimit()

	err := http.ListenAndServe(CONFIG.VStr("host"), nil)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
