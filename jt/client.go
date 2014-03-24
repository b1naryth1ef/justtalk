package jt

import (
	"./json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/kennygrant/sanitize"
	"github.com/russross/blackfriday"
	"log"
	"strings"
)

type Connection struct {
	ws       *websocket.Conn
	Buffer   chan []byte
	Username string
	Name     string
	Avatar   string
	Channels []*Channel
	Limit    int
	Afk      bool
}

func (c *Connection) LoadUser(u json.Object) {
	c.Username = u.VStr("email")
	c.Name = u.VStr("given_name")
	c.Avatar = u.VStr("picture")
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
		c.Send(json.Object{
			"type":    "hello",
			"success": false,
			"msg":     "You are already logged in!",
		})
		return
	}

	CHANS["lobby"].Join(c)
	CONNS[c.Username] = c
	c.Afk = false

	c.Send(json.Object{
		"type":    "hello",
		"success": true,
		"user":    c.ToJson(),
	})
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
	// Prevent darren from haxing
	msg := sanitize.HTML(o.VStr("msg"))

	if msg == "" {
		return
	}

	if msg[0] == '/' {
		c.ActionCmd(ch, o)
		return
	}

	// Prevent yogi from spamming
	if len(msg) > 1000 {
		c.SendS(ChatError{
			Msg: "That message is too large!",
		})
		return
	}

	packet := json.Object{
		"avatar":   c.Avatar,
		"username": c.Username,
		"name":     c.Name,
		"msg":      string(blackfriday.MarkdownCommon([]byte(msg))),
		"raw":      msg,
		"dest":     ch.Name,
		"type":     "msg",
	}

	data, _ := packet.Dump()
	RED.Publish("justtalk-"+ch.Name, string(data))

	ch.SendRaw(packet)
}

func (c *Connection) SendImage(ch *Channel, url string) {
	packet := json.Object{
		"avatar":   c.Avatar,
		"username": c.Username,
		"name":     c.Name,
		"msg":      fmt.Sprintf(`<a href="%s" target="_blank"><img class="embed" src="%s" /></a>`, url, url),
		"raw":      url,
		"dest":     ch.Name,
		"type":     "msg",
		"nofloat":  true,
	}

	data, _ := packet.Dump()
	RED.Publish("justtalk-"+ch.Name, string(data))
	ch.SendRaw(packet)
}

func (c *Connection) ActionJoin(o json.Object) {
	var ch *Channel
	var has bool
	chans := o.Value("channels").([]interface{})
	for _, v := range chans {
		val, err := v.(string)
		if !err {
			return
		}
		chan_name := sanitize.HTML(val)
		log.Printf("Channel: %s", chan_name)
		ch, has = CHANS[chan_name]
		if !has {
			if len(FindChannel(chan_name)) > 0 {
				if len(CHANS) > 1024 {
					log.Printf("Limiting number of channels!")
					return
				}
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

func (c *Connection) ActionAfk(o json.Object) {
	if !o.Has("state") {
		return
	}

	c.Afk = o.Value("state").(bool)
	for _, v := range CONNS {
		v.Send(json.Object{
			"type":  "afk",
			"user":  c.Username,
			"state": c.Afk,
		})
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

		// Rate limit users
		if c.Limit < 35 {
			c.Limit += 1
		} else {
			log.Printf("Rate limiting %s", c.Username)
			continue
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

			// User cannot send to a channel that they are not a member of
			if _, ch := dest.Members[c.Username]; !ch {
				resp.Set("type", "error")
				resp.Set("msg", fmt.Sprintf("Not member of channel: %s", destname))
				c.Send(resp)
				continue
			}
		}

		if action == "hello" {
			c.ActionHello(obj)
		} else if action == "msg" {
			if (dest) == nil {
				continue
			}
			c.ActionMsg(dest, obj)
		} else if action == "join" {
			c.ActionJoin(obj)
		} else if action == "afk" {
			c.ActionAfk(obj)
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

func (c Connection) ToJson() json.Object {
	return json.Object{
		"type":     "user",
		"username": c.Username,
		"name":     c.Name,
		"avatar":   c.Avatar,
	}
}
