package jt

import (
	"./json"
	"crypto/md5"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/gorilla/websocket"
	"github.com/russross/blackfriday"
	"github.com/vmihailenco/redis/v2"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type CmdFunc func(*Connection, *Channel, json.Object, []string)

var CONNS map[string]*Connection = make(map[string]*Connection, 0)
var CHANS map[string]*Channel = make(map[string]*Channel, 0)
var CMDS map[string]CmdFunc = make(map[string]CmdFunc, 0)
var DB *db.DB
var RED *redis.Client

func Bind(n string, f CmdFunc) {
	CMDS[n] = f
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err == nil {
		c := &Connection{
			Buffer: make(chan []byte, 256),
			ws:     ws}
		go c.ReadLoop()
		c.WriteLoop()
	} else {
		log.Printf("handleWebSocket err: %v", err)
		http.Error(w, "Something went wrong yo!", 500)
	}
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
}

func setup_redis() {
	RED = redis.NewTCPClient(&redis.Options{
		Addr: "localhost:6379",
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
	err := r.ParseMultipartForm(100000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	m := r.MultipartForm

	files := m.File["file"]
	for i, _ := range files {
		log.Printf("Parsing one file!")
		file, err := files[i].Open()
		defer file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func Run() {
	setup_db()
	setup_redis()
	hook_exit()
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/socket", handleWebSocket)
	http.HandleFunc("/api/send", web_send_to)
	http.HandleFunc("/api/upload", web_upload)

	Bind("join", func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /join <channel>"})
			return
		}
		chan_name := strings.ToLower(args[1])

		if _, chan_exists := CHANS[chan_name]; !chan_exists {
			channel := NewChannel(chan_name, chan_name, "", getAvatarUrl(chan_name))
			CHANS[chan_name] = channel
		}

		CHANS[chan_name].Join(u)
	})

	Bind("quit", func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /quit <channel>"})
			return
		}
		chan_name := strings.ToLower(args[1])

		for _, ch := range u.Channels {
			if ch.Name == chan_name {
				ch.Quit(u, "%s has left the channel")
				return
			}
		}

		u.SendS(ChatError{
			Msg: fmt.Sprintf("You are not part of the channel '%s'", chan_name),
		})
	})

	Bind("delete", func(u *Connection, c *Channel, o json.Object, args []string) {
		if c.Name == "lobby" {
			u.Send(json.Object{
				"type": "error",
				"msg":  "You cannot delete the lobby!",
			})
			return
		}
		c.Delete()
	})

	Bind("cset", func(u *Connection, c *Channel, o json.Object, args []string) {
		resp := make(json.Object)
		if len(args) < 3 {
			u.SendS(ChatError{Msg: "Usage: /cset <option> <value>"})
			return
		}

		resp.Set("type", "updatechannel")
		resp.Set("name", c.Name)

		if args[1] == "topic" {
			c.Topic = string(blackfriday.MarkdownCommon([]byte(strings.Join(args[2:], " "))))
			resp.Set("k", "topic")
			resp.Set("v", c.Topic)
			resp.Set("a", fmt.Sprintf("%s has changed the channel topic", u.Name))
		} else if args[1] == "image" {
			c.Image = args[2]
			resp.Set("k", "image")
			resp.Set("v", c.Image)
			resp.Set("a", fmt.Sprintf("%s has changed the channel icon", u.Name))
		} else if args[1] == "title" {
			c.Title = strings.Join(args[2:], " ")
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

	})

	CHANS["lobby"] = NewChannel("lobby", "The Lobby", "Sit down and have a cup of tea", "http://media2.hpcwire.com/datanami/couchbase.png")

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
