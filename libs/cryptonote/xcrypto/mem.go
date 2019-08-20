package xcrypto

import "sync"

type objectID int32

var refs struct {
	sync.Mutex
	objs map[objectID]interface{}
	nect objectID
}

func init() {
	refs.Lock()
	refs.objs = make(map[objectID]interface{})
	refs.nect = 1000
	refs.Unlock()
}

func put(obj interface{}) objectID {
	refs.Lock()
	id := refs.nect
	refs.nect++
	refs.objs[id] = obj
	refs.Unlock()
	return id
}

func get(id objectID) interface{} {
	refs.Lock()
	obj := refs.objs[id]
	refs.Unlock()
	return obj
}

func free(id objectID) interface{} {
	refs.Lock()
	obj := refs.objs[id]
	delete(refs.objs, id)
	refs.Unlock()
	return obj
}
