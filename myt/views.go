package main

import (
	"sync/atomic"

	"gopkg.in/kataras/iris.v6/adaptors/websocket"
)

type pageView struct {
	source string
	count  uint64
}

func (v *pageView) increment() {
	atomic.AddUint64(&v.count, 1)
}

func (v *pageView) decrement() {
	oldCount := v.count
	if oldCount > 0 {
		atomic.StoreUint64(&v.count, oldCount-1)
	}
}

func (v *pageView) getCount() uint64 {
	val := atomic.LoadUint64(&v.count)
	return val
}

type (
	pageViews []pageView
)

func (v *pageViews) Add(source string) {
	args := *v
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.source == source {
			kv.increment()
			return
		}
	}

	c := cap(args)
	if c > n {
		args = args[:n+1]
		kv := &args[n]
		kv.source = source
		kv.count = 1
		*v = args
		return
	}

	kv := pageView{}
	kv.source = source
	kv.count = 1
	*v = append(args, kv)
}

func (v *pageViews) Get(source string) *pageView {
	args := *v
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.source == source {
			return kv
		}
	}
	return nil
}

func (v *pageViews) GetOrSet(source string) *pageView {
	args := *v
	n := len(args)
	for i := 0; i < n; i++ {
		kv := &args[i]
		if kv.source == source {
			return kv
		}
	}
	kv := pageView{source: source, count: 0}
	*v = append(args, kv)
	return &kv
}

func (v *pageViews) Reset() {
	*v = (*v)[:0]
}

var v pageViews

func HandleWebsocketConnection(c websocket.Connection) {

	c.On("watch", func(pageSource string) {
		//	pagesView := v.GetOrSet(pageSource)
		// pagesView.increment()
		v.Add(pageSource)
		// join the socket to a room linked with the page source
		c.Join(pageSource)

		viewsCount := v.Get(pageSource).getCount()
		if viewsCount == 0 {
			viewsCount++ // count should be always > 0 here
		}
		c.To(pageSource).Emit("watch", viewsCount)
	})

	c.OnLeave(func(roomName string) {
		if roomName != c.ID() { // if the roomName  it's not the connection iself
			// the roomName here is the source, this is the only room(except the connection's ID room) which we join the users to.
			pageV := v.Get(roomName)
			if pageV == nil {
				return // for any case that this room is not a pageView source
			}
			// decrement -1 the specific counter for this page source.
			pageV.decrement()
			// 1. open 30 tabs.
			// 2. close the browser.
			// 3. re-open the browser
			// 4. should be  v.getCount() = 1
			// in order to achieve the previous flow we should decrement exactly when the user disconnects
			// but emit the result a little after, on a goroutine
			// getting all connections within this room and emit the online views one by one.
			// note:
			// we can also add a time.Sleep(2-3 seconds) inside the goroutine at the future.
			go func(currentConnID string) {
				for _, conn := range ws.GetConnectionsByRoom(roomName) {
					if conn.ID() != currentConnID {
						conn.Emit("watch", pageV.getCount())
					}

				}
			}(c.ID())
		}

	})

}
