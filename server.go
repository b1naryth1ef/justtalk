package main

import (
	"./json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	//"strings"
)

type Connection struct {
	ws       *websocket.Conn
	Buffer   chan []byte
	Username string
	Avatar   string
}

type Channel struct {
	Name    string
	Title   string
	Topic   string
	Members []*Connection
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

type CmdFunc func(*Connection, *Channel, json.Object)

var CONNS []*Connection = make([]*Connection, 0)
var CHANS map[string]*Channel = make(map[string]*Channel, 0)
var CMDS map[string]CmdFunc

func NewChannel(name, title string) *Channel {
	return &Channel{
		Name:    name,
		Title:   title,
		Topic:   "Yolo Swag!",
		Members: make([]*Connection, 0),
	}
}

func (c *Channel) Join(u *Connection) {
	c.Members = append(c.Members, u)
	c.Send(ChatAction{
		Icon:   "user",
		Action: fmt.Sprintf("%s has joined %s", u.Username, c.Title),
		Dest:   c.Name,
	})
}

func (c *Channel) Send(s Serializable) {
	for _, user := range c.Members {
		user.Send(s.ToJson())
	}
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
		"username": c.User.Username,
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
	resp := make(json.Object, 0)
	resp.Set("type", "hello")
	resp.Set("success", true)
	c.Username = o.VStr("username")
	c.Avatar = "http://www.gravatar.com/avatar/94d093eda664addd61350d7e98281bc3d?s=32&d=identicon&r=PG"
	c.Send(resp)
	CHANS["lobby"].Join(c)
}

func (c *Connection) ActionCmd(ch *Channel, o json.Object) {
	cmd_name := o.VStr("msg")[1:]
	if cmd, che := CMDS[cmd_name]; che {
		cmd(c, ch, o)
	} else {
		c.SendS(ChatError{
			Msg: fmt.Sprintf("Unknown command: %s!", cmd_name),
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

func (c *Connection) ReadLoop() {
	var dest *Channel
	var ch bool

	defer c.ws.Close()
	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			log.Printf("ReadLoop err: %v", err)
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
		CONNS = append(CONNS, c)
		go c.ReadLoop()
		c.WriteLoop()
	} else {
		log.Printf("handleWebSocket err: %v", err)
		http.Error(w, "Something went wrong yo!", 500)
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/socket", handleWebSocket)

	CHANS["lobby"] = NewChannel("lobby", "The Lobby")

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
