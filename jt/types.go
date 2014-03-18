package jt

import "./json"

type ChatError struct {
	Msg string
}

type Serializable interface {
	ToJson() json.Object
}

func (c ChatError) ToJson() json.Object {
	return json.Object{
		"type": "error",
		"msg":  c.Msg,
	}
}
