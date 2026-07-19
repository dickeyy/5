package generallogging

import (
	"sync"
	"time"
)

// CachedMessage is bounded edit/delete context and is never treated as a permanent archive.
type CachedMessage struct {
	GuildID, ChannelDiscordID, MessageDiscordID, AuthorDiscordUserID, Content string
	Attachments                                                               []AttachmentMetadata
	EmbedTypes                                                                []string
	CachedAt                                                                  time.Time
}

// MessageCache is a concurrency-safe per-guild FIFO cache with independently configurable limits.
type MessageCache struct {
	mu           sync.Mutex
	guilds       map[string]*guildCache
	limits       map[string]int
	defaultLimit int
}
type guildCache struct {
	order    []string
	messages map[string]CachedMessage
}

// NewMessageCache constructs a bounded cache and normalizes unsafe defaults.
func NewMessageCache(defaultLimit int) *MessageCache {
	if defaultLimit < 1 {
		defaultLimit = 1000
	}
	return &MessageCache{guilds: map[string]*guildCache{}, limits: map[string]int{}, defaultLimit: defaultLimit}
}

// SetGuildLimit changes one guild's cap and immediately evicts oldest entries.
func (c *MessageCache) SetGuildLimit(guildID string, limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if limit < 1 {
		limit = 1
	}
	c.limits[guildID] = limit
	c.evict(guildID)
}

// Put stores or replaces one message while preserving stable FIFO order.
func (c *MessageCache) Put(message CachedMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.guilds[message.GuildID]
	if g == nil {
		g = &guildCache{messages: map[string]CachedMessage{}}
		c.guilds[message.GuildID] = g
	}
	if _, ok := g.messages[message.MessageDiscordID]; !ok {
		g.order = append(g.order, message.MessageDiscordID)
	}
	g.messages[message.MessageDiscordID] = cloneMessage(message)
	c.evict(message.GuildID)
}

// Get returns a defensive copy of cached message context.
func (c *MessageCache) Get(guildID, messageID string) (CachedMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.guilds[guildID]
	if g == nil {
		return CachedMessage{}, false
	}
	message, ok := g.messages[messageID]
	return cloneMessage(message), ok
}

// Delete removes and returns one cached message.
func (c *MessageCache) Delete(guildID, messageID string) (CachedMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	g := c.guilds[guildID]
	if g == nil {
		return CachedMessage{}, false
	}
	message, ok := g.messages[messageID]
	if !ok {
		return CachedMessage{}, false
	}
	delete(g.messages, messageID)
	for i, id := range g.order {
		if id == messageID {
			g.order = append(g.order[:i], g.order[i+1:]...)
			break
		}
	}
	return cloneMessage(message), true
}

// Len returns one guild's current bounded cache size.
func (c *MessageCache) Len(guildID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.guilds[guildID] == nil {
		return 0
	}
	return len(c.guilds[guildID].messages)
}
func (c *MessageCache) evict(guildID string) {
	g := c.guilds[guildID]
	if g == nil {
		return
	}
	limit := c.limits[guildID]
	if limit == 0 {
		limit = c.defaultLimit
	}
	for len(g.order) > limit {
		id := g.order[0]
		g.order = g.order[1:]
		delete(g.messages, id)
	}
}
func cloneMessage(m CachedMessage) CachedMessage {
	m.Attachments = append([]AttachmentMetadata(nil), m.Attachments...)
	m.EmbedTypes = append([]string(nil), m.EmbedTypes...)
	return m
}
