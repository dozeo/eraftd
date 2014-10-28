package eraftd

type ClusterBackend interface {
	Write(in []string) ([]string, error)
	Read(in []string) ([]string, error)
	Save() ([]byte, error)
	Recovery([]byte) error
}
