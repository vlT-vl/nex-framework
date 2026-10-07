package app

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var errSecondInstance = errors.New("second instance notified")

type singleInstance struct {
	ln         net.Listener
	secretPath string
}

type singleInstanceState struct {
	Addr   string `json:"addr"`
	Secret string `json:"secret"`
}

type singleInstanceMessage struct {
	Secret string   `json:"secret"`
	Args   []string `json:"args"`
}

func (a *App) startSingleInstance() error {
	id := strings.TrimSpace(a.cfg.SingleInstanceID)
	if id == "" {
		return nil
	}
	addr := singleInstanceAddr(id)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		state, readErr := readSingleInstanceState(id)
		if readErr == nil && state.Addr != "" && notifySecondInstance(state.Addr, state.Secret, os.Args) == nil {
			return errSecondInstance
		}
		if notifySecondInstance(addr, "", os.Args) == nil {
			return errSecondInstance
		}
		return err
	}
	secret, err := randomHex(32)
	if err != nil {
		_ = ln.Close()
		return err
	}
	secretPath := singleInstanceSecretPath(id)
	if err := writeSingleInstanceState(secretPath, singleInstanceState{Addr: addr, Secret: secret}); err != nil {
		_ = ln.Close()
		return err
	}
	a.single = &singleInstance{ln: ln, secretPath: secretPath}
	go a.acceptSecondInstances(ln, secret)
	return nil
}

func (a *App) stopSingleInstance() {
	if a.single != nil && a.single.ln != nil {
		_ = a.single.ln.Close()
	}
	if a.single != nil && a.single.secretPath != "" {
		_ = os.Remove(a.single.secretPath)
	}
}

func (a *App) acceptSecondInstances(ln net.Listener, secret string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
			var msg singleInstanceMessage
			_ = json.NewDecoder(bufio.NewReader(c)).Decode(&msg)
			if secret != "" && subtle.ConstantTimeCompare([]byte(msg.Secret), []byte(secret)) != 1 {
				_, _ = c.Write([]byte("nex-single-instance-forbidden\n"))
				return
			}
			_, _ = c.Write([]byte("nex-single-instance-ok\n"))
			if a.cfg.OnSecondInstance != nil {
				a.cfg.OnSecondInstance(a, msg.Args)
			}
			a.Emit("app.second-instance", map[string]any{"args": msg.Args})
		}(conn)
	}
}

func notifySecondInstance(addr, secret string, args []string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(singleInstanceMessage{Secret: secret, Args: args}); err != nil {
		return err
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(line) != "nex-single-instance-ok" {
		return errors.New("single instance handshake failed")
	}
	return nil
}

func writeSingleInstanceState(path string, state singleInstanceState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func readSingleInstanceState(id string) (singleInstanceState, error) {
	var state singleInstanceState
	b, err := os.ReadFile(singleInstanceSecretPath(id))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return state, err
	}
	return state, nil
}

func singleInstanceAddr(id string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	port := 49152 + int(h.Sum32()%12000)
	return net.JoinHostPort("127.0.0.1", strconvItoa(port))
}

func singleInstanceSecretPath(id string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return filepath.Join(os.TempDir(), "nex-single-"+strconvItoa(int(h.Sum32()))+".json")
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func strconvItoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
