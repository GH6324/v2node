package panel

import (
	"path"
)

// Debug set the client debug for client
func (c *Client) Debug() {
	c.client.SetDebug(true)
}

func (c *Client) assembleURL(p string) string {
	return path.Join(c.APIHost + p)
}
