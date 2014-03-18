package jt

import "./json"
import "log"
import "github.com/HouzuoGuo/tiedot/db"
import "fmt"
import jzon "encoding/json"

type Channel struct {
	ID      uint64
	Name    string
	Title   string
	Topic   string
	Image   string
	Members map[string]*Connection
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
}

func (c *Channel) Quit(u *Connection, msg string) {
	obj := make(json.Object)
	obj.Set("type", "quit")
	obj.Set("name", c.Name)
	obj.Set("user", u.Username)
	c.SendRaw(obj)
	delete(c.Members, u.Username)
}

func (c *Channel) Join(u *Connection) {
	c.Members[u.Username] = u
	u.Channels = append(u.Channels, c)
	obj := make(json.Object)
	obj.Set("type", "join")
	obj.Set("name", c.Name)
	obj.Set("user", u.ToJson())
	c.SendRaw(obj)
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
