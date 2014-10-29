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
	"time"

	"github.com/goraft/raft"
	"github.com/zenazn/goji/web"
	"github.com/zenazn/goji/web/util"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Eraftd struct {
	name       string
	host       string
	port       int
	path       string
	router     *web.Mux
	raft       raft.Server
	httpServer *http.Server
	backend    ClusterBackend
}

func StartCluster(port int, host string, join string, cb ClusterBackend, path string) *Eraftd {
	raft.RegisterCommand(&DistributedWrite{})
	if err := os.MkdirAll(path, 0744); err != nil {
		Logger.Fatalf("Unable to create path '%s': %v", path, err)
	}

	// Creates a new server.
	s := &Eraftd{
		host:    host,
		port:    port,
		path:    path,
		backend: cb,
		router:  web.New(),
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
func (s *Eraftd) connectionString() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

func (s *Eraftd) connectionStringLeader() string {
	var leaderID string
	for leaderID == "" {
		leaderID = s.raft.Leader()
		if leaderID == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
	lp, ok := s.raft.Peers()[leaderID]
	if ok {
		return lp.ConnectionString
	}
	if leaderID == s.raft.Name() {
		return ""
	}
	Logger.Println("WARNING: RETRY LEADER SEARCH ", leaderID)
	time.Sleep(1 * time.Second)
	return s.connectionStringLeader()
}

// Starts the server.
func (s *Eraftd) ListenAndServe(leader string) error {
	var err error

	Logger.Printf("Initializing Raft Server: %s", s.path)

	// Initialize and start Raft server.
	httpTransport := raft.NewHTTPTransporter("/raft", 200*time.Millisecond)
	s.raft, err = raft.NewServer(s.name, s.path, httpTransport, s.backend, s.backend, "")
	if err != nil {
		Logger.Fatal(err)
	}
	httpTransport.Install(s.raft, s)
	s.raft.Start()

	hasjoin := false

	if leader != "" {
		// Join to leader if specified.
		Logger.Println("Attempting to join leader:", leader)

		if !s.raft.IsLogEmpty() {
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
	if !hasjoin && s.raft.IsLogEmpty() {
		// Initialize the server by joining itself.

		Logger.Println("Initializing new cluster")

		_, err := s.raft.Do(&raft.DefaultJoinCommand{
			Name:             s.raft.Name(),
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

	// Register a filtered Logger for the http server
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
	s.HandleFunc("/connection", s.stringToHandler(s.connectionString))
	s.HandleFunc("/connectionLeader", s.stringToHandler(s.connectionStringLeader))

	go s.httpServer.ListenAndServe()

	return nil
}

func (s *Eraftd) stringToHandler(f func() string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(f()))
		r.Body.Close()
	}
}

// HandleFunc() interface.
func (s *Eraftd) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.Handle(pattern, handler)
}

// Joins to the leader of an existing cluster.
func (s *Eraftd) Join(leader string) error {
	command := &raft.DefaultJoinCommand{
		Name:             s.raft.Name(),
		ConnectionString: s.connectionString(),
	}

	var buffer bytes.Buffer
	json.NewEncoder(&buffer).Encode(command)
	resp, err := http.Post(fmt.Sprintf("http://%s/join", leader), "application/json", &buffer)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

func (s *Eraftd) infoHandler(w http.ResponseWriter, req *http.Request) {
	if s.raft.Leader() != s.raft.Name() {
		w.Write([]byte("ERROR: This Server is not the leader"))
		w.Write([]byte("\n"))
		w.Write([]byte("The leader is "))
		w.Write([]byte(s.connectionStringLeader()))
		w.Write([]byte("\n"))
	} else {
		w.Write([]byte("This Server is leader !"))
		w.Write([]byte("\n"))
	}
	for _, p := range s.raft.Peers() {
		w.Write([]byte(p.ConnectionString))
		w.Write([]byte(" "))
		w.Write([]byte(strconv.FormatInt(time.Now().Unix()-p.LastActivity().Unix(), 10)))
		w.Write([]byte("\n"))
	}
}

func (s *Eraftd) joinHandler(w http.ResponseWriter, req *http.Request) {
	Logger.Println("received join request")
	if s.raft.Leader() != s.raft.Name() {
		// we are a read only node => redirect request to other node
		master := s.raft.Peers()[s.raft.Leader()].ConnectionString
		Logger.Println("MASTER:", master)
		Logger.Println("redirect to master: ", master)
		url, _ := url.Parse(master + "")
		Logger.Println("redirect to url:", url)
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(w, req)
		return
	}
	// needed to join a cluster
	djc := &raft.DefaultJoinCommand{}

	if err := json.NewDecoder(req.Body).Decode(&djc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.raft.Do(djc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Eraftd) writeHandler(w http.ResponseWriter, req *http.Request) {
	master := s.connectionStringLeader()
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
	wc := DistributedWrite{}
	err = json.Unmarshal(b, &wc)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Execute the command against the Raft server.
	ok, err := s.raft.Do(raft.Command(&wc))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	bytes, err := json.Marshal(ok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	w.Write(bytes)
}

func (s *Eraftd) Write(in []string) ([]string, error) {
	wc := NewDistributedWrite(in)
	master := s.connectionStringLeader()
	if master == "" {
		a, b := s.raft.Do(wc)
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
		rbody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		sa := []string{}
		err = json.Unmarshal(rbody, &sa)
		if err != nil {
			return nil, err
		}
		return sa, nil
	}
}

func (s *Eraftd) Read(in []string) ([]string, error) {
	return s.backend.Read(in)
}

func (s *Eraftd) Save() ([]byte, error) {
	return s.backend.Save()
}

func (s *Eraftd) Recovery(in []byte) error {
	return s.backend.Recovery(in)
}
