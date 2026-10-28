// Vendored from github.com/zserge/lorca v0.1.10 (MIT, see LICENSE).
//
// Bridge-Ground changes (Issue #15): Edge sometimes relaunches itself right
// after start, so the process we spawned exits (stderr EOF) while the
// relaunched browser opens the window and the DevTools port. To survive that:
//   - the DevTools URL is taken from stderr or from <user-data-dir>/DevToolsActivePort,
//     whichever appears first, and our process exiting is not treated as failure
//   - c.done is closed when the DevTools connection ends, not when our process exits
//   - kill() asks the browser to close via Browser.close before killing our process

package lorca

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

// devToolsTimeout bounds how long newChromeWithArgs waits for the DevTools
// endpoint to become available.
const devToolsTimeout = 15 * time.Second

type h = map[string]interface{}

// Result is a struct for the resulting value of the JS expression or an error.
type result struct {
	Value json.RawMessage
	Err   error
}

type bindingFunc func(args []json.RawMessage) (interface{}, error)

// Msg is a struct for incoming messages (results and async events)
type msg struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type chrome struct {
	sync.Mutex
	cmd      *exec.Cmd
	ws       *websocket.Conn
	id       int32
	target   string
	session  string
	window   int
	pending  map[int]chan result
	bindings map[string]bindingFunc
	done     chan struct{} // closed when the DevTools connection ends
	doneOnce sync.Once
}

func newChromeWithArgs(chromeBinary, userDataDir string, args ...string) (*chrome, error) {
	// The first two IDs are used internally during the initialization
	c := &chrome{
		id:       2,
		pending:  map[int]chan result{},
		bindings: map[string]bindingFunc{},
		done:     make(chan struct{}),
	}

	// A stale file from an earlier run would point at a dead port.
	activePortFile := filepath.Join(userDataDir, "DevToolsActivePort")
	_ = os.Remove(activePortFile)

	// Start chrome process
	c.cmd = exec.Command(chromeBinary, args...)
	pipe, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, err
	}
	go c.cmd.Wait()

	wsURL, err := waitForDevTools(pipe, activePortFile, devToolsTimeout)
	if err != nil {
		c.kill()
		return nil, err
	}

	// Open a websocket
	c.ws, err = websocket.Dial(wsURL, "", "http://127.0.0.1")
	if err != nil {
		c.kill()
		return nil, err
	}

	// Find target and initialize session
	c.target, err = c.findTarget()
	if err != nil {
		c.kill()
		return nil, err
	}

	c.session, err = c.startSession(c.target)
	if err != nil {
		c.kill()
		return nil, err
	}
	go c.readLoop()
	for method, args := range map[string]h{
		"Page.enable":          nil,
		"Target.setAutoAttach": {"autoAttach": true, "waitForDebuggerOnStart": false},
		"Network.enable":       nil,
		"Runtime.enable":       nil,
		"Security.enable":      nil,
		"Performance.enable":   nil,
		"Log.enable":           nil,
	} {
		if _, err := c.send(method, args); err != nil {
			c.kill()
			return nil, err
		}
	}

	if !contains(args, "--headless") {
		win, err := c.getWindowForTarget(c.target)
		if err != nil {
			c.kill()
			return nil, err
		}
		c.window = win.WindowID
	}

	return c, nil
}

func (c *chrome) findTarget() (string, error) {
	err := websocket.JSON.Send(c.ws, h{
		"id": 0, "method": "Target.setDiscoverTargets", "params": h{"discover": true},
	})
	if err != nil {
		return "", err
	}
	for {
		m := msg{}
		if err = websocket.JSON.Receive(c.ws, &m); err != nil {
			return "", err
		} else if m.Method == "Target.targetCreated" {
			target := struct {
				TargetInfo struct {
					Type string `json:"type"`
					ID   string `json:"targetId"`
				} `json:"targetInfo"`
			}{}
			if err := json.Unmarshal(m.Params, &target); err != nil {
				return "", err
			} else if target.TargetInfo.Type == "page" {
				return target.TargetInfo.ID, nil
			}
		}
	}
}

func (c *chrome) startSession(target string) (string, error) {
	err := websocket.JSON.Send(c.ws, h{
		"id": 1, "method": "Target.attachToTarget", "params": h{"targetId": target},
	})
	if err != nil {
		return "", err
	}
	for {
		m := msg{}
		if err = websocket.JSON.Receive(c.ws, &m); err != nil {
			return "", err
		} else if m.ID == 1 {
			if m.Error != nil {
				return "", errors.New("Target error: " + string(m.Error))
			}
			session := struct {
				ID string `json:"sessionId"`
			}{}
			if err := json.Unmarshal(m.Result, &session); err != nil {
				return "", err
			}
			return session.ID, nil
		}
	}
}

// WindowState defines the state of the Chrome window, possible values are
// "normal", "maximized", "minimized" and "fullscreen".
type WindowState string

const (
	// WindowStateNormal defines a normal state of the browser window
	WindowStateNormal WindowState = "normal"
	// WindowStateMaximized defines a maximized state of the browser window
	WindowStateMaximized WindowState = "maximized"
	// WindowStateMinimized defines a minimized state of the browser window
	WindowStateMinimized WindowState = "minimized"
	// WindowStateFullscreen defines a fullscreen state of the browser window
	WindowStateFullscreen WindowState = "fullscreen"
)

// Bounds defines settable window properties.
type Bounds struct {
	Left        int         `json:"left"`
	Top         int         `json:"top"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	WindowState WindowState `json:"windowState"`
}

type windowTargetMessage struct {
	WindowID int    `json:"windowId"`
	Bounds   Bounds `json:"bounds"`
}

func (c *chrome) getWindowForTarget(target string) (windowTargetMessage, error) {
	var m windowTargetMessage
	msg, err := c.send("Browser.getWindowForTarget", h{"targetId": target})
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(msg, &m)
	return m, err
}

type targetMessageTemplate struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
	Params struct {
		Name    string `json:"name"`
		Payload string `json:"payload"`
		ID      int    `json:"executionContextId"`
		Args    []struct {
			Type  string      `json:"type"`
			Value interface{} `json:"value"`
		} `json:"args"`
	} `json:"params"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Result json.RawMessage `json:"result"`
}

type targetMessage struct {
	targetMessageTemplate
	Result struct {
		Result struct {
			Type        string          `json:"type"`
			Subtype     string          `json:"subtype"`
			Description string          `json:"description"`
			Value       json.RawMessage `json:"value"`
			ObjectID    string          `json:"objectId"`
		} `json:"result"`
		Exception struct {
			Exception struct {
				Value json.RawMessage `json:"value"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	} `json:"result"`
}

func (c *chrome) readLoop() {
	defer c.doneOnce.Do(func() { close(c.done) })
	for {
		m := msg{}
		if err := websocket.JSON.Receive(c.ws, &m); err != nil {
			return
		}

		if m.Method == "Target.receivedMessageFromTarget" {
			params := struct {
				SessionID string `json:"sessionId"`
				Message   string `json:"message"`
			}{}
			json.Unmarshal(m.Params, &params)
			if params.SessionID != c.session {
				continue
			}
			res := targetMessage{}
			json.Unmarshal([]byte(params.Message), &res)

			if res.ID == 0 && res.Method == "Runtime.consoleAPICalled" || res.Method == "Runtime.exceptionThrown" {
				log.Println(params.Message)
			} else if res.ID == 0 && res.Method == "Runtime.bindingCalled" {
				payload := struct {
					Name string            `json:"name"`
					Seq  int               `json:"seq"`
					Args []json.RawMessage `json:"args"`
				}{}
				json.Unmarshal([]byte(res.Params.Payload), &payload)

				c.Lock()
				binding, ok := c.bindings[res.Params.Name]
				c.Unlock()
				if ok {
					jsString := func(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
					go func() {
						result, error := "", `""`
						if r, err := binding(payload.Args); err != nil {
							error = jsString(err.Error())
						} else if b, err := json.Marshal(r); err != nil {
							error = jsString(err.Error())
						} else {
							result = string(b)
						}
						expr := fmt.Sprintf(`
							if (%[4]s) {
								window['%[1]s']['errors'].get(%[2]d)(%[4]s);
							} else {
								window['%[1]s']['callbacks'].get(%[2]d)(%[3]s);
							}
							window['%[1]s']['callbacks'].delete(%[2]d);
							window['%[1]s']['errors'].delete(%[2]d);
							`, payload.Name, payload.Seq, result, error)
						c.send("Runtime.evaluate", h{"expression": expr, "contextId": res.Params.ID})
					}()
				}
				continue
			}

			c.Lock()
			resc, ok := c.pending[res.ID]
			delete(c.pending, res.ID)
			c.Unlock()

			if !ok {
				continue
			}

			if res.Error.Message != "" {
				resc <- result{Err: errors.New(res.Error.Message)}
			} else if res.Result.Exception.Exception.Value != nil {
				resc <- result{Err: errors.New(string(res.Result.Exception.Exception.Value))}
			} else if res.Result.Result.Type == "object" && res.Result.Result.Subtype == "error" {
				resc <- result{Err: errors.New(res.Result.Result.Description)}
			} else if res.Result.Result.Type != "" {
				resc <- result{Value: res.Result.Result.Value}
			} else {
				res := targetMessageTemplate{}
				json.Unmarshal([]byte(params.Message), &res)
				resc <- result{Value: res.Result}
			}
		} else if m.Method == "Target.targetDestroyed" {
			params := struct {
				TargetID string `json:"targetId"`
			}{}
			json.Unmarshal(m.Params, &params)
			if params.TargetID == c.target {
				c.kill()
				return
			}
		}
	}
}

func (c *chrome) send(method string, params h) (json.RawMessage, error) {
	id := atomic.AddInt32(&c.id, 1)
	b, err := json.Marshal(h{"id": int(id), "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	resc := make(chan result)
	c.Lock()
	c.pending[int(id)] = resc
	c.Unlock()

	if err := websocket.JSON.Send(c.ws, h{
		"id":     int(id),
		"method": "Target.sendMessageToTarget",
		"params": h{"message": string(b), "sessionId": c.session},
	}); err != nil {
		return nil, err
	}
	res := <-resc
	return res.Value, res.Err
}

func (c *chrome) load(url string) error {
	_, err := c.send("Page.navigate", h{"url": url})
	return err
}

func (c *chrome) eval(expr string) (json.RawMessage, error) {
	return c.send("Runtime.evaluate", h{"expression": expr, "awaitPromise": true, "returnByValue": true})
}

func (c *chrome) bind(name string, f bindingFunc) error {
	c.Lock()
	// check if binding already exists
	_, exists := c.bindings[name]

	c.bindings[name] = f
	c.Unlock()

	if exists {
		// Just replace callback and return, as the binding was already added to js
		// and adding it again would break it.
		return nil
	}

	if _, err := c.send("Runtime.addBinding", h{"name": name}); err != nil {
		return err
	}
	script := fmt.Sprintf(`(() => {
	const bindingName = '%s';
	const binding = window[bindingName];
	window[bindingName] = async (...args) => {
		const me = window[bindingName];
		let errors = me['errors'];
		let callbacks = me['callbacks'];
		if (!callbacks) {
			callbacks = new Map();
			me['callbacks'] = callbacks;
		}
		if (!errors) {
			errors = new Map();
			me['errors'] = errors;
		}
		const seq = (me['lastSeq'] || 0) + 1;
		me['lastSeq'] = seq;
		const promise = new Promise((resolve, reject) => {
			callbacks.set(seq, resolve);
			errors.set(seq, reject);
		});
		binding(JSON.stringify({name: bindingName, seq, args}));
		return promise;
	}})();
	`, name)
	_, err := c.send("Page.addScriptToEvaluateOnNewDocument", h{"source": script})
	if err != nil {
		return err
	}
	_, err = c.eval(script)
	return err
}

func (c *chrome) setBounds(b Bounds) error {
	if b.WindowState == "" {
		b.WindowState = WindowStateNormal
	}
	param := h{"windowId": c.window, "bounds": b}
	if b.WindowState != WindowStateNormal {
		param["bounds"] = h{"windowState": b.WindowState}
	}
	_, err := c.send("Browser.setWindowBounds", param)
	return err
}

func (c *chrome) bounds() (Bounds, error) {
	result, err := c.send("Browser.getWindowBounds", h{"windowId": c.window})
	if err != nil {
		return Bounds{}, err
	}
	bounds := struct {
		Bounds Bounds `json:"bounds"`
	}{}
	err = json.Unmarshal(result, &bounds)
	return bounds.Bounds, err
}

func (c *chrome) pdf(width, height int) ([]byte, error) {
	result, err := c.send("Page.printToPDF", h{
		"paperWidth":  float32(width) / 96,
		"paperHeight": float32(height) / 96,
	})
	if err != nil {
		return nil, err
	}
	pdf := struct {
		Data []byte `json:"data"`
	}{}
	err = json.Unmarshal(result, &pdf)
	return pdf.Data, err
}

func (c *chrome) png(x, y, width, height int, bg uint32, scale float32) ([]byte, error) {
	if x == 0 && y == 0 && width == 0 && height == 0 {
		// By default either use SVG size if it's an SVG, or use A4 page size
		bounds, err := c.eval(`document.rootElement ? [document.rootElement.x.baseVal.value, document.rootElement.y.baseVal.value, document.rootElement.width.baseVal.value, document.rootElement.height.baseVal.value] : [0,0,816,1056]`)
		if err != nil {
			return nil, err
		}
		rect := make([]int, 4)
		if err := json.Unmarshal(bounds, &rect); err != nil {
			return nil, err
		}
		x, y, width, height = rect[0], rect[1], rect[2], rect[3]
	}

	_, err := c.send("Emulation.setDefaultBackgroundColorOverride", h{
		"color": h{
			"r": (bg >> 16) & 0xff,
			"g": (bg >> 8) & 0xff,
			"b": bg & 0xff,
			"a": (bg >> 24) & 0xff,
		},
	})
	if err != nil {
		return nil, err
	}
	result, err := c.send("Page.captureScreenshot", h{
		"clip": h{
			"x": x, "y": y, "width": width, "height": height, "scale": scale,
		},
	})
	if err != nil {
		return nil, err
	}
	pdf := struct {
		Data []byte `json:"data"`
	}{}
	err = json.Unmarshal(result, &pdf)
	return pdf.Data, err
}

func (c *chrome) kill() error {
	if c.ws != nil {
		// The browser may be a relaunched process we don't own, so killing
		// c.cmd alone would leave its window open. Ask it to close itself.
		id := atomic.AddInt32(&c.id, 1)
		_ = websocket.JSON.Send(c.ws, h{"id": int(id), "method": "Browser.close"})
		_ = c.ws.Close()
	}
	// TODO: cancel all pending requests
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill() // fails harmlessly if it already exited
	}
	return nil
}

// waitForDevTools returns the browser's DevTools websocket URL, read from
// either the "DevTools listening on" stderr line or the DevToolsActivePort
// file in the profile directory. Our process exiting (stderr EOF) is not an
// error, since a relaunched browser still writes the file.
func waitForDevTools(stderr io.ReadCloser, activePortFile string, timeout time.Duration) (string, error) {
	found := make(chan string, 1)
	go func() {
		re := regexp.MustCompile(`^DevTools listening on (ws://.*?)\r?\n$`)
		br := bufio.NewReader(stderr)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				stderr.Close()
				return
			}
			if m := re.FindStringSubmatch(line); m != nil {
				found <- m[1]
				io.Copy(ioutil.Discard, br)
				return
			}
		}
	}()

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(timeout)
	for {
		select {
		case u := <-found:
			return u, nil
		case <-tick.C:
			if u, ok := readActivePortFile(activePortFile); ok {
				return u, nil
			}
		case <-deadline:
			return "", fmt.Errorf("DevTools endpoint not available within %s", timeout)
		}
	}
}

// readActivePortFile parses DevToolsActivePort ("<port>\n<path>") into a
// websocket URL. It reports false until the browser has written both lines.
func readActivePortFile(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		return "", false
	}
	port, wsPath := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	if port == "" || !strings.HasPrefix(wsPath, "/devtools/browser/") {
		return "", false
	}
	return "ws://127.0.0.1:" + port + wsPath, true
}

func contains(arr []string, x string) bool {
	for _, n := range arr {
		if x == n {
			return true
		}
	}
	return false
}
