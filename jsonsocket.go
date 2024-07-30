package apirouter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

var (
	jsonClients   = make(map[uuid.UUID]*jsonclient)
	jsonClientsLk sync.RWMutex
)

// BroadcastJson sends a message to ALL peers connected to the json socket. It should be formatted with
// at least something similar to: map[string]any{"result": "event", "data": ...}
func BroadcastJson(ctx context.Context, data any) error {
	clients := listJsonClients()
	for _, c := range clients {
		go c.Encode(data)
	}
	return nil
}

func listJsonClients() []*jsonclient {
	jsonClientsLk.RLock()
	defer jsonClientsLk.RUnlock()

	res := make([]*jsonclient, 0, len(jsonClients))
	for _, c := range jsonClients {
		res = append(res, c)
	}
	return res
}

// MakeJsonSocketFD returns a file descriptor (integer) for a new json socket
func MakeJsonSocketFD(extraObjects map[string]any) (int, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to create socket pair: %w", err)
	}

	f := os.NewFile(uintptr(fds[1]), "pipe")
	defer f.Close()
	c, err := net.FileConn(f)
	if err != nil {
		return -1, fmt.Errorf("failed to handle socket: %w", err)
	}

	go handleJsonClient(c, extraObjects)

	return fds[0], nil
}

// MakeJsonUnixListener creates a UNIX socket at the given path and listen to it, initializing a json socket for each
// connection.
// It uses some tricks if socketName is too long, since there is a 104 chars limits on darwin and 108 chars limit on linux
func MakeJsonUnixListener(socketName string, extraObjects map[string]any) error {
	socketName, err := filepath.Abs(socketName)
	if err != nil {
		return err
	}
	// create a socket at path socketName
	os.Remove(socketName)
	if d := filepath.Dir(socketName); d != "." {
		os.MkdirAll(filepath.Dir(socketName), 0755)
	}
	abs, err := filepath.Abs(socketName)
	if err != nil {
		return err
	}
	dataDir := filepath.Dir(abs)

	if len(socketName) >= 100 {
		// there's a risk we are at the limit of what's acceptable (104 on ios, 108 on android), let's create a temp name
		for {
			// use a relative name in current dir, this allows ios to work
			// sample temp path on ios: /Users/name/Library/Developer/CoreSimulator/Devices/<uuid>/data/Containers/Data/Application/<another_uuid>/tmp
			// we do not chdir, assuming the current (default) dir is writable
			if wd, err := os.Getwd(); err != nil {
				log.Printf("failed to get wd, things may be going wrong: %s", err)
				os.Chdir(dataDir)
			} else {
				log.Printf("using cwd for temp socket, cwd=%s", wd)
			}
			tp := fmt.Sprintf(".socket_tmp.%d.%d", os.Getpid(), rand.Uint64())
			if _, err := os.Lstat(tp); err == nil {
				continue
			}
			err = os.Symlink(socketName, tp)
			if err != nil {
				// possible that wd isn't writable
				os.Chdir(dataDir)
				err = os.Symlink(socketName, tp)
				if err != nil {
					return err
				}
			}
			defer os.Remove(tp)
			socketName = tp
			break
		}
	}
	s, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketName})
	if err != nil {
		return err
	}
	// TODO if there is an error make sure directory is writable, attempt to chdir to data dir if not?

	go listenJsonSocket(s, extraObjects)

	return nil
}

// listenJsonSocket listens to the given listener and instanciates a socket for each new connection
func listenJsonSocket(l net.Listener, extraObjects map[string]any) {
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("listen failed: %s", err)
			return
		}
		go handleJsonClient(c, extraObjects)
	}
}

type jsonclient struct {
	c   net.Conn
	enc *json.Encoder
	wlk sync.Mutex // write lock
	id  uuid.UUID
}

func (cl *jsonclient) Encode(obj any) error {
	cl.wlk.Lock()
	defer cl.wlk.Unlock()

	return cl.enc.Encode(obj)
}

func (cl *jsonclient) SendResponse(r *Response) error {
	return cl.Encode(r)
}

func (cl *jsonclient) run(obj *Context) {
	resp, _ := obj.Response()
	err := cl.SendResponse(resp)
	if err != nil {
		log.Printf("failed to write response: %s", err)
		cl.c.Close()
	}
}

func (cl *jsonclient) register() {
	jsonClientsLk.Lock()
	defer jsonClientsLk.Unlock()

	jsonClients[cl.id] = cl
}

func (cl *jsonclient) deregister() {
	jsonClientsLk.Lock()
	defer jsonClientsLk.Unlock()

	delete(jsonClients, cl.id)
}

// handleJsonClient is a goroutine that handles one end of the socket pair.
func handleJsonClient(c net.Conn, extraObjects map[string]any) {
	defer c.Close()

	defer func() {
		if e := recover(); e != nil {
			log.Printf("recovered from panic in json client: %s", e)
		}
	}()

	cl := &jsonclient{
		c:   c,
		enc: json.NewEncoder(c),
		id:  uuid.Must(uuid.NewRandom()),
	}
	cl.register()
	defer cl.deregister()

	dec := json.NewDecoder(c)

	for {
		obj := New(context.Background(), "", "")
		if extraObjects != nil {
			for k, v := range extraObjects {
				obj.SetObject(k, v)
			}
		}
		obj.SetObject("@client", cl)
		obj.SetResponseSink(cl)

		// SetDecoder will block to read and set context state based on one object read from the decoder
		err := obj.SetDecoder(dec)
		if err != nil {
			log.Printf("failed to decode json request received from RPC: %s", err)
			return
		}
		// execute in background
		go cl.run(obj)
	}
}
