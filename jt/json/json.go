package json

import "encoding/json"
import "fmt"

type Object map[string]interface{}
type IterFunc func(bool, string, interface{})

func (o Object) Dump() ([]byte, error) {
	return json.Marshal(o)
}

func (o Object) IsObj(s string) bool {
	if !o.Has(s) {
		return false
	}
	switch o[s].(type) {
	case Object:
		return true
	default:
		return false
	}
	return false
}

func (o Object) Has(s string) bool {
	_, has := o[s]
	return has
}

func (o Object) Del(s string) {
	delete(o, s)
}

func (o Object) Set(s string, v interface{}) {
	o[s] = v
}

func (o Object) HasValues(s ...string) bool {
	for _, v := range s {
		if !o.Has(v) {
			return false
		}
	}
	return true
}

func (o Object) Value(s string) interface{} {
	if o.Has(s) {
		return o[s]
	}
	return nil
}

func (o Object) VStr(s string) string {
	if o.Has(s) && !o.IsObj(s) {
		v, e := o.Value(s).(string)
		if !e {
			return ""
		}
		return v
	}
	return ""
}

func (o Object) Get(s string) Object {
	if o.Has(s) {
		return o[s].(Object)
	}
	return nil
}

func (o Object) Iter(f IterFunc) {
	for k, v := range o {
		f(o.IsObj(k), k, v)
	}
}

func Loader(c map[string]interface{}) Object {
	fresh := Object{}
	for k, v := range c {
		switch v.(type) {
		case map[string]interface{}:
			fresh[k] = Loader(v.(map[string]interface{}))
		default:
			fresh[k] = v
		}
	}
	return fresh
}

func LoadJsonFromArray(val []byte) []Object {
	j := make([]map[string]interface{}, 0)
	if err := json.Unmarshal(val, &j); err != nil {
		fmt.Printf("%s", err)
	}
	result := make([]Object, 0)
	for _, item := range j {
		result = append(result, Loader(item))
	}
	return result
}

func LoadJson(val []byte) Object {
	j := make(map[string]interface{}, 0)
	if err := json.Unmarshal(val, &j); err != nil {
		fmt.Printf("%s", err)
	}

	return Loader(j)
}
