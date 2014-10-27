package eraftd

import "github.com/goraft/raft"

// This command writes a value to a key.
type WriteCommand struct {
	Data []string `json:"data"`
}

// Creates a new write command.
func NewWriteCommand(in []string) *WriteCommand {
	return &WriteCommand{
		Data: in,
	}
}

// The name of the command in the log.
func (c *WriteCommand) CommandName() string {
	return "write"
}

// Writes a value to a key.
func (c *WriteCommand) Apply(server raft.Server) (interface{}, error) {
	db := server.Context().(ClusterBackend)
	return db.Write(c.Data)
}
