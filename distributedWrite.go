package eraftd

import "github.com/goraft/raft"

// This command writes a value to a key.
type DistributedWrite struct {
	Data []string `json:"data"`
}

// Creates a new write command.
func NewDistributedWrite(in []string) *DistributedWrite {
	return &DistributedWrite{
		Data: in,
	}
}

// The name of the command in the log.
func (c *DistributedWrite) CommandName() string {
	return "write"
}

// Writes a value to a key.
func (c *DistributedWrite) Apply(server raft.Server) (interface{}, error) {
	db := server.Context().(ClusterBackend)
	return db.Write(c.Data)
}
