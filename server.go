package main

import (
	"./json"
	"crypto/md5"
	jzon "encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type Connection struct {
	ws       *websocket.Conn
	Buffer   chan []byte
	Username string
	Name     string
	Avatar   string
	Channels []*Channel
}

type Channel struct {
	ID      uint64
	Name    string
	Title   string
	Topic   string
	Image   string
	Members map[string]*Connection
}

type ChatMessage struct {
	User *Connection
	Msg  string
	Dest string
}

type ChatAction struct {
	Icon   string
	Action string
	Dest   string
}

type ChatError struct {
	Msg string
}

type Serializable interface {
	ToJson() json.Object
}

type CmdFunc func(*Connection, *Channel, json.Object, []string)

var CONNS map[string]*Connection = make(map[string]*Connection, 0)
var CHANS map[string]*Channel = make(map[string]*Channel, 0)
var CMDS map[string]CmdFunc = make(map[string]CmdFunc, 0)
var DB *db.DB

func Bind(n string, f CmdFunc) {
	CMDS[n] = f
}

func NewChannel(name, title, topic, image string) *Channel {
	ch := Channel{
		Name:    name,
		Title:   title,
		Topic:   topic,
		Image:   image,
		Members: make(map[string]*Connection, 0),
	}
	if !ch.Load() {
		log.Printf("Creating completely new channel!")
		ch.SaveNew()
	} else {
		ch.Save()
	}
	return &ch
}

func (c *Channel) Delete() {
	chans := DB.Use("channels")
	chans.Delete(c.ID)

	obj := make(json.Object)
	obj.Set("type", "channelclose")
	obj.Set("name", c.Name)
	c.SendRaw(obj)
	delete(CHANS, c.Name)
}

func (c *Channel) Save() {
	chans := DB.Use("channels")
	err := chans.Update(c.ID, c.ToJson())
	if err != nil {
		panic(err)
	}
	log.Printf("Saved channel %s!", c.ID)
}

func (c *Channel) SaveNew() {
	chans := DB.Use("channels")
	doc, err := chans.Insert(c.ToJson())
	if err != nil {
		panic(err)
	}
	log.Printf("Added channel %s!", doc)
	c.ID = doc
}

func FindChannel(name string) map[uint64]struct{} {
	chans := DB.Use("channels")
	// so securezzz
	queryStr := fmt.Sprintf(`[{"eq": "%s", "in": ["name"]}]`, name)
	var query interface{}
	jzon.Unmarshal([]byte(queryStr), &query)

	queryResult := make(map[uint64]struct{})
	if err := db.EvalQuery(query, chans, &queryResult); err != nil {
		return nil
	}
	log.Printf("Size: %s", len(queryResult))
	return queryResult
}

func (c *Channel) Load() bool {
	chans := DB.Use("channels")
	queryResult := FindChannel(c.Name)
	if len(queryResult) > 0 {
		var data interface{}
		var id uint64
		for id = range queryResult {
			chans.Read(id, &data)
			fmt.Printf("Query returned document %v\n", data)
		}
		newdata := json.Loader(data.(map[string]interface{}))
		c.ID = id
		c.Title = newdata.VStr("title")
		c.Topic = newdata.VStr("topic")
		c.Image = newdata.VStr("image")
		return true
	}
	return false
	// val, _ := query.Dump()
	// log.Printf("Query: %s", val)
}

func (c *Channel) Quit(u *Connection, msg string) {
	c.Send(ChatAction{
		Icon:   "user",
		Action: msg,
		Dest:   c.Name,
	})
	delete(c.Members, u.Username)
}

func (c *Channel) Join(u *Connection) {
	c.Members[u.Username] = u
	u.Channels = append(u.Channels, c)
	c.Send(ChatAction{
		Icon:   "user",
		Action: fmt.Sprintf("%s has joined %s", u.Name, c.Title),
		Dest:   c.Name,
	})
	u.Send(c.ToJson())
}

func (c *Channel) IsMember(u *Connection) bool {
	_, is := c.Members[u.Username]
	return is
}

func (c *Channel) Send(s Serializable) {
	c.SendRaw(s.ToJson())
}

func (c *Channel) SendRaw(o json.Object) {
	for _, user := range c.Members {
		user.Send(o)
	}
}

func (c Connection) ToJson() json.Object {
	return json.Object{
		"type":     "user",
		"username": c.Username,
		"name":     c.Name,
		"avatar":   c.Avatar,
	}
}

func (c Channel) ToJson() json.Object {
	obj := json.Object{
		"type":  "channel",
		"name":  c.Name,
		"title": c.Title,
		"topic": c.Topic,
		"image": c.Image,
	}
	var members []json.Object = make([]json.Object, 0)
	for _, mem := range c.Members {
		members = append(members, mem.ToJson())
	}
	obj.Set("members", members)
	return obj
}

func (c ChatError) ToJson() json.Object {
	return json.Object{
		"type": "error",
		"msg":  c.Msg,
	}
}

func (c ChatAction) ToJson() json.Object {
	return json.Object{
		"icon":   c.Icon,
		"action": c.Action,
		"dest":   c.Dest,
		"type":   "action",
	}
}

func (c ChatMessage) ToJson() json.Object {
	return json.Object{
		"avatar":   c.User.Avatar,
		"username": c.User.Name,
		"msg":      c.Msg,
		"dest":     c.Dest,
		"type":     "msg",
	}
}

func (c *Connection) Send(o json.Object) {
	data, _ := o.Dump()
	c.ws.WriteMessage(websocket.TextMessage, data)
}

func (c *Connection) SendS(o Serializable) {
	c.Send(o.ToJson())
}

func (c *Connection) ActionHello(o json.Object) {
	if _, ch := CONNS[o.VStr("username")]; ch {
		c.SendS(ChatError{Msg: "Nope!"})
		return
	}

	resp := make(json.Object, 0)
	resp.Set("type", "hello")
	resp.Set("success", true)
	c.Username = o.VStr("username")
	c.Name = o.VStr("name")
	c.Avatar = getAvatarUrl(c.Username)
	resp.Set("avatar", c.Avatar)
	c.Send(resp)
	CHANS["lobby"].Join(c)
	CONNS[c.Username] = c
}

func (c *Connection) ActionCmd(ch *Channel, o json.Object) {
	args := strings.Split(o.VStr("msg")[1:], " ")
	if cmd, che := CMDS[args[0]]; che {
		cmd(c, ch, o, args)
	} else {
		c.SendS(ChatError{
			Msg: fmt.Sprintf("Unknown command: '%s'!", args[0]),
		})
	}
}

func (c *Connection) ActionMsg(ch *Channel, o json.Object) {
	msg := o.VStr("msg")

	if msg == "" {
		return
	}

	if msg[0] == '/' {
		c.ActionCmd(ch, o)
		return
	}

	ch.Send(ChatMessage{
		User: c,
		Msg:  o.VStr("msg"),
		Dest: ch.Name,
	})
}

func (c *Connection) ActionJoin(o json.Object) {
	var ch *Channel
	var has bool
	chans := o.Value("channels").([]interface{})
	for _, v := range chans {
		chan_name := v.(string)
		log.Printf("Channel: %s", chan_name)
		ch, has = CHANS[chan_name]
		if !has {
			if len(FindChannel(chan_name)) > 0 {
				ch = NewChannel(chan_name, chan_name, "", "")
				CHANS[ch.Name] = ch
			} else {
				continue
			}
		}
		if !ch.IsMember(c) {
			ch.Join(c)
		}

	}
}

func (c *Connection) Disconnect(msg string) {
	delete(CONNS, c.Username)
	for _, v := range c.Channels {
		v.Quit(c, fmt.Sprintf(msg, c.Name))
	}
}

func (c *Connection) ReadLoop() {
	var dest *Channel
	var ch bool

	defer c.ws.Close()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			log.Printf("Read Loop Err: %v", err)
			c.Disconnect("%s has quit")
			return
		}

		log.Printf("Got message: %s", message)

		obj := json.LoadJson(message)
		action := obj.VStr("type")
		resp := make(json.Object, 0)

		if obj.Has("dest") {
			destname := obj.VStr("dest")
			if dest, ch = CHANS[destname]; !ch {
				resp.Set("type", "error")
				resp.Set("msg", fmt.Sprintf("Invalid channel: %s", destname))
				c.Send(resp)
				continue
			}
		}

		if action == "hello" {
			c.ActionHello(obj)
		} else if action == "msg" {
			c.ActionMsg(dest, obj)
		} else if action == "join" {
			c.ActionJoin(obj)
		}
	}
}

func (c *Connection) WriteLoop() {
	defer c.ws.Close()
	for message := range c.Buffer {
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("WriteLoop err: %v", err)
			return
		}
	}
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
			os.Exit(0)
		}
	}()
}

func setup_db() {
	rand.Seed(time.Now().UTC().UnixNano())
	new_db, err := db.OpenDB("sweg")
	if err != nil {
		panic(err)
	}
	DB = new_db

	DB.Create("channels", 2)
	chans := DB.Use("channels")
	chans.Index([]string{"name"})
}

func main() {
	setup_db()
	hook_exit()
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/socket", handleWebSocket)

	Bind("join", func(u *Connection, c *Channel, o json.Object, args []string) {
		if len(args) < 2 {
			u.SendS(ChatError{Msg: "Usage: /join <channel>"})
			return
		}
		if _, chan_exists := CHANS[args[1]]; !chan_exists {
			channel := NewChannel(args[1], args[1], "", getAvatarUrl(args[1]))
			CHANS[args[1]] = channel
		}

		CHANS[args[1]].Join(u)
	})

	Bind("delete", func(u *Connection, c *Channel, o json.Object, args []string) {
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
			c.Topic = strings.Join(args[2:], " ")
			resp.Set("k", "topic")
			resp.Set("v", c.Topic)
			resp.Set("a", fmt.Sprintf("%s changed the topic to '%s'", u.Name, c.Topic))
		} else if args[1] == "image" {
			c.Image = args[2]
			resp.Set("k", "image")
			resp.Set("v", c.Image)
			resp.Set("a", fmt.Sprintf("%s changed the channel icon", u.Name))
		} else if args[1] == "title" {
			c.Title = strings.Join(args[2:], " ")
			if len(c.Title) > 25 {
				c.Title = c.Title[:25]
			}
			resp.Set("k", "title")
			resp.Set("v", c.Title)
			resp.Set("a", fmt.Sprintf("%s changed the title to '%s'", u.Name, c.Title))
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
