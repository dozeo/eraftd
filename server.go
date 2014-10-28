package eraftd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goraft/raft"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/util"
)

type Server struct {
	name       string
	host       string
	port       int
	path       string
	router     *web.Mux
	raftServer raft.Server
	httpServer *http.Server
	db         ClusterBackend
	mutex      sync.RWMutex
}

func StartCluster(port int, host string, join string, cb ClusterBackend, path string) *Server {
	raft.RegisterCommand(&WriteCommand{})
	if err := os.MkdirAll(path, 0744); err != nil {
		Logger.Fatalf("Unable to create path: %v", err)
	}

	// Creates a new server.
	s := &Server{
		host:   host,
		port:   port,
		path:   path,
		db:     cb,
		router: web.New(),
	}

	// Read existing name or generate a new one.
	if b, err := ioutil.ReadFile(filepath.Join(path, "name")); err == nil {
		s.name = string(b)
	} else {
		s.name = fmt.Sprintf("%07x", rand.Int())[0:7]
		if err = ioutil.WriteFile(filepath.Join(path, "name"), []byte(s.name), 0644); err != nil {
			panic(err)
		}
	}
	s.ListenAndServe(join)
	return s
}

// Returns the connection string.
func (s *Server) connectionString() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

func (s *Server) connectionStringMaster() string {
	var leader string
	for leader == "" {
		leader = s.raftServer.Leader()
		if leader == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	lp, ok := s.raftServer.Peers()[leader]
	if ok {
		return lp.ConnectionString
	}
	if leader == s.raftServer.Name() {
		return ""
	}
	Logger.Println("WARNING: RETRY LEADER SEARCH ", leader)
	time.Sleep(1 * time.Second)
	return s.connectionStringMaster()
}

// Starts the server.
func (s *Server) ListenAndServe(leader string) error {
	var err error

	Logger.Printf("Initializing Raft Server: %s", s.path)

	// Initialize and start Raft server.
	transporter := raft.NewHTTPTransporter("/raft", 200*time.Millisecond)
	s.raftServer, err = raft.NewServer(s.name, s.path, transporter, s.db, s.db, "")
	if err != nil {
		Logger.Fatal(err)
	}
	transporter.Install(s.raftServer, s)
	s.raftServer.Start()

	hasjoin := false

	if leader != "" {
		// Join to leader if specified.

		Logger.Println("Attempting to join leader:", leader)

		if !s.raftServer.IsLogEmpty() {
			Logger.Print("Cannot join with an existing")
			Logger.Print("WARNING: Will try to join old clusterg")
		} else {
			if err := s.Join(leader); err != nil {
				Logger.Println("Cannot join Leader: " + err.Error())
			} else {
				Logger.Println("Cluster joined leader: ", leader)
				hasjoin = true
			}
		}

	}
	if !hasjoin && s.raftServer.IsLogEmpty() {
		// Initialize the server by joining itself.

		Logger.Println("Initializing new cluster")

		_, err := s.raftServer.Do(&raft.DefaultJoinCommand{
			Name:             s.raftServer.Name(),
			ConnectionString: s.connectionString(),
		})
		if err != nil {
			Logger.Fatal(err)
		}

	} else {
		Logger.Println("Recovered from log")
	}

	Logger.Println("Initializing HTTP server")

	// Initialize and start HTTP server.
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	//s.router.Use(middleware.Logger)
	s.router.Use(func(c *web.C, h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lw := util.WrapWriter(w)
			h.ServeHTTP(lw, r)
			if !strings.Contains(r.URL.String(), "raft") {
				Logger.Println("\033[32m", r.Method, r.URL.String(), lw.Status(), "\033[0m")
			}
		})
	})
	s.HandleFunc("/join", s.joinHandler)
	s.HandleFunc("/write", s.writeHandler)
	s.HandleFunc("/info", s.infoHandler)

	go s.httpServer.ListenAndServe()

	return nil
}

// This is a hack around Gorilla mux not providing the correct net/http
// HandleFunc() interface.
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.Handle(pattern, handler)
}

// Joins to the leader of an existing cluster.
func (s *Server) Join(leader string) error {
	command := &raft.DefaultJoinCommand{
		Name:             s.raftServer.Name(),
		ConnectionString: s.connectionString(),
	}

	var b bytes.Buffer
	json.NewEncoder(&b).Encode(command)
	resp, err := http.Post(fmt.Sprintf("http://%s/join", leader), "application/json", &b)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func (s *Server) infoHandler(w http.ResponseWriter, req *http.Request) {
	if s.raftServer.Leader() != s.raftServer.Name() {
		w.Write([]byte("ERROR: This Server is not the leader"))
		w.Write([]byte("\n"))
		w.Write([]byte("The leader is "))
		w.Write([]byte(s.connectionStringMaster()))
		w.Write([]byte("\n"))
		//return
	} else {
		w.Write([]byte("This Server is leader !"))
		w.Write([]byte("\n"))
	}
	for _, p := range s.raftServer.Peers() {
		w.Write([]byte(p.ConnectionString))
		w.Write([]byte(" "))
		w.Write([]byte(strconv.FormatInt(time.Now().Unix()-p.LastActivity().Unix(), 10)))
		w.Write([]byte("\n"))
	}
}

func (s *Server) joinHandler(w http.ResponseWriter, req *http.Request) {
	Logger.Println("received join request")
	if s.raftServer.Leader() != s.raftServer.Name() {
		// we are a read only node => redirect request to other node
		master := s.raftServer.Peers()[s.raftServer.Leader()].ConnectionString
		Logger.Println("MASTER:", master)
		Logger.Println("redirect to master: ", master)
		url, _ := url.Parse(master + "")
		Logger.Println("redirect to url:", url)
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(w, req)
		return
	}
	// needed to join a cluster
	jcommand := &raft.DefaultJoinCommand{}

	if err := json.NewDecoder(req.Body).Decode(&jcommand); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.raftServer.Do(jcommand); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

/*
func (s *Server) readHandler(w http.ResponseWriter, req *http.Request) {
	value, _ := s.db.Read([]string{"k1"})
	w.Write([]byte(value[0]))
}
*/
func (s *Server) writeHandler(w http.ResponseWriter, req *http.Request) {
	master := s.connectionStringMaster()
	if master != "" {
		// we are a read only node => redirect request to other node
		// could happen while leader changes
		Logger.Println("WRITE HANDLER REDIRECT TO MASTER SHOULD NOT HAPPEN: ", master)
		url, _ := url.Parse(master)
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(w, req)
		return
	}
	// Read the value from the POST body.
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	wc := WriteCommand{}
	err = json.Unmarshal(b, &wc)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Execute the command against the Raft server.
	ok, err := s.raftServer.Do(raft.Command(&wc))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	bytes, err := json.Marshal(ok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.Write(bytes)
}

func (s *Server) Write(in []string) ([]string, error) {
	wc := NewWriteCommand(in)
	master := s.connectionStringMaster()
	if master == "" {
		a, b := s.raftServer.Do(wc)
		c := a.([]string)
		return c, b
	} else {
		// needs to be redirected to the master
		jbytes, err := json.Marshal(wc)
		if err != nil {
			return nil, err
		}
		req, err := http.Post(master+"/write", "text/json", bytes.NewBuffer(jbytes))
		if err != nil {
			return nil, err
		}
		br, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		sa := []string{}
		err = json.Unmarshal(br, &sa)
		if err != nil {
			return nil, err
		}
		return sa, nil
	}
}

func (s *Server) Read(in []string) ([]string, error) {
	return s.db.Read(in)
}

func (s *Server) Save() ([]byte, error) {
	return s.db.Save()
}

func (s *Server) Recovery(in []byte) error {
	return s.db.Recovery(in)
}
