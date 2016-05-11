package imagecache

import "go4.org/syncutil/singleflight"

type parallelGroup struct {
	ch chan struct{}
	g  singleflight.Group
}

func newParallelGroup(n int) *parallelGroup {
	if n < 1 {
		n = 1
	}
	return &parallelGroup{ch: make(chan struct{}, n)}
}

func (p *parallelGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	return p.g.Do(key, func() (interface{}, error) {
		p.ch <- struct{}{}
		defer func() {
			<-p.ch
		}()
		return fn()
	})
}
