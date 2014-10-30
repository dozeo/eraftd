package eraftd

import (
	"fmt"

	"github.com/goraft/raft"
)

// This command writes a value to a key.
type DistributedRemove struct {
	Host string
}

// Creates a new write command.
func NewDistributedRemove(host string) *DistributedRemove {
	return &DistributedRemove{
		Host: host,
	}
}

// The name of the command in the log.
func (c *DistributedRemove) CommandName() string {
	return "remove"
}

// Writes a value to a key.
func (c *DistributedRemove) Apply(server raft.Server) (interface{}, error) {
	fmt.Println("DistributedRemove", " ", c.Host)
	err := server.RemovePeer(c.Host)
	return nil, err
}
