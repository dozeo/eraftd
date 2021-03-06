eraftd
======

raft cluster helper based on https://github.com/goraft/raft.

This package is intended for embedding in applications which need a small shared storage

Storage is not implemented in this package.

The Storage just needs to implement this simple interface:
```
type ClusterBackend interface {
        Write(in []string) ([]string, error)
        Read(in []string) ([]string, error)
        Save() ([]byte, error)
        Recovery([]byte) error
}
```  
There is an example implementation in the examples folder, which also contains a sample key-value store.

To start a new Cluster simply call
```
cDB := eraftd.StartCluster(4001, "localhost","localhost:4002", db, "/tmp/node.1")
                           ^ local port
                                  ^ local hostname
                                              ^ existing node (if there is any)
                                                               ^ ClusterBackend_Implementation
                                                                   ^ data folder for raft
```
Now you can read and write to your cluster over cDB, which implements the ClusterBackend interface

All calls to cDB.Write() will be replicated over the whole cluster. Calls to cDB.Read() are handled by the local storage.
