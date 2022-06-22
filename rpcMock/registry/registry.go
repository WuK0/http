package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

func (r *Registry) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		writer.Header().Set("X-rpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := req.Header.Get("X-rpc-Servers")
		if addr == "" {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type ServerItem struct {
	Addr  string
	start time.Time
}

const (
	defaultPath    = "/_rpc_/registry"
	defaultTimeout = time.Minute * 5
)

var DefaultRegistry = New(defaultTimeout)

func New(timeout time.Duration) *Registry {
	return &Registry{
		servers: map[string]*ServerItem{},
		timeout: timeout,
	}
}

func (r *Registry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		s.start = time.Now()
	}
}
func (r *Registry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]string, 0)
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			res = append(res, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(res)
	return res
}

func (r *Registry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}
func HandleHTTP() {
	DefaultRegistry.HandleHTTP(defaultPath)
}

func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}
func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-rpc-Servers", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil
}
