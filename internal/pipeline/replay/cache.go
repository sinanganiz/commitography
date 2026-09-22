package replay

import "container/list"

// cacheBytes bounds the file contents replay keeps between reads. A version a
// commit writes is usually the version a later commit changes, and aligning
// needs it again; holding the most recent ones saves reading them twice. It
// changes nothing but how often the object reader is asked.
const cacheBytes = 16 << 20

// contentCache holds recently read contents by object name, least recently
// used out first, within a byte budget.
type contentCache struct {
	budget, used int
	order        *list.List // of *cached, most recent at the front
	byName       map[string]*list.Element
}

type cached struct {
	name    string
	content []byte
}

func newContentCache(budget int) *contentCache {
	return &contentCache{budget: budget, order: list.New(), byName: map[string]*list.Element{}}
}

func (c *contentCache) get(name string) ([]byte, bool) {
	element, ok := c.byName[name]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*cached).content, true
}

// put keeps a content, unless it alone is over the budget.
func (c *contentCache) put(name string, content []byte) {
	if _, ok := c.byName[name]; ok || len(content) > c.budget {
		return
	}
	c.byName[name] = c.order.PushFront(&cached{name: name, content: content})
	c.used += len(content)
	for c.used > c.budget {
		oldest := c.order.Back()
		entry := oldest.Value.(*cached)
		c.order.Remove(oldest)
		delete(c.byName, entry.name)
		c.used -= len(entry.content)
	}
}
