package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"syscall"
	"time"

	"coffee.mort.mediator/platform"
	"coffee.mort.mediator/screencap"
	"github.com/BurntSushi/toml"
	"github.com/go-vgo/robotgo"
	"nhooyr.io/websocket"
)

type Config struct {
	BasePath string `toml:"base_path"`
	ScrollStep int `toml:"scroll_step"`
}

type EmptyData struct {}

type WSMessage struct {
	Type string `json:"type"`
	Data json.RawMessage `json:"data"`
}

type KeyboardTypeData struct {
	Text string `json:"text"`
}

type KeyboardKeyData struct {
	Key string `json:"key"`
	Modifiers []string `json:"modifiers"`
}

type ScrollData struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type MouseClickData struct {
	Button string `json:"button"`
	DoubleClick bool `json:"doubleClick"`
}

type MouseButtonData struct {
	Button string `json:"button"`
}

type MousePosData struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ScreenSizeData struct {
	Width int `json:"width"`
	Height int `json:"height"`
}

type DirEntryData struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type ListDirData struct {
	Entries []DirEntryData `json:"entries"`
}

type Error struct {
	Error string `json:"error"`
}

func readConfig() (*Config, error) {
	confFile, err := os.Open("config.toml")
	if err != nil {
		return nil, err
	}
	defer confFile.Close()

	var conf Config
	_, err = toml.NewDecoder(confFile).Decode(&conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

type RW http.ResponseWriter
type Req http.Request

func handler(h func(w RW, req *Req) error) (
		func(w http.ResponseWriter, req *http.Request)) {
	return func(w http.ResponseWriter, req *http.Request) {
		err := h(RW(w), (*Req)(req))
		if err != nil {
			w.WriteHeader(400)
			err = json.NewEncoder(w).Encode(&Error{err.Error()})
			if err != nil {
				w.Write([]byte("Oh no, failed to encode error struct"))
			}
		}
	}
}

func main() {
	conf, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Config: %#v", conf)

	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	handleWebsocketMessage := func(msg *WSMessage) error {
		var err error
		if msg.Type == "mouse-move" {
			var pos MousePosData
			err = json.Unmarshal(msg.Data, &pos)
			if err != nil {
				return err
			}

			robotgo.MoveMouse(pos.X, pos.Y)
			return nil
		} else if msg.Type == "mouse-click" {
			var click MouseClickData
			err = json.Unmarshal(msg.Data, &click)
			if err != nil {
				return err
			}

			robotgo.MouseClick(click.Button, click.DoubleClick)
			return nil
		} else if msg.Type == "mouse-down" {
			var btn MouseButtonData
			err = json.Unmarshal(msg.Data, &btn)
			if err != nil {
				return err
			}

			robotgo.Toggle(btn.Button, "down")
			return nil
		} else if msg.Type == "mouse-up" {
			var btn MouseButtonData
			err = json.Unmarshal(msg.Data, &btn)
			if err != nil {
				return err
			}

			robotgo.Toggle(btn.Button, "up")
			return nil
		} else if msg.Type == "scroll" {
			var scroll ScrollData
			err = json.Unmarshal(msg.Data, &scroll)
			if err != nil {
				return err
			}

			robotgo.Scroll(scroll.X * conf.ScrollStep, scroll.Y * conf.ScrollStep)
			return nil
		} else if msg.Type == "keyboard-type" {
			var text KeyboardTypeData
			err = json.Unmarshal(msg.Data, &text)
			if err != nil {
				return err
			}

			robotgo.TypeStr(text.Text)
			return nil
		} else if msg.Type == "keyboard-key" {
			var key KeyboardKeyData
			err = json.Unmarshal(msg.Data, &key)
			if err != nil {
				return err
			}

			var modifiers []interface{}
			for _, modifier := range key.Modifiers {
				modifiers = append(modifiers, modifier)
			}

			robotgo.KeyTap(key.Key, modifiers...)
			return nil
		} else {
			return fmt.Errorf("Unknown message type: %s", msg.Type)
		}
	}

	http.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Println("Error accepting websocket:", err)
			return
		}

		defer c.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			done := ctx.Done()
			pos := MousePosData{X: -1, Y: -1}

			for {
				select {
				case <-time.After(33 * time.Millisecond):
				case <-done:
					return
				}

				x, y := robotgo.GetMousePos()
				if x == pos.X && y == pos.Y {
					continue
				}

				pos.X = x
				pos.Y = y
				posData, err := json.Marshal(&pos)
				if err != nil {
					log.Println("Marshal error:", err)
					return
				}

				data, err := json.Marshal(&WSMessage{
					Type: "mouse-move",
					Data: posData,
				})
				if err != nil {
					log.Println("Marshal error:", err)
					return
				}

				err = c.Write(ctx, websocket.MessageText, data)
				if err != nil {
					log.Println("Write error:", err)
					return
				}
			}
		}()

		for {
			_, buf, err := c.Read(ctx)
			if err == io.EOF {
				return
			} else if err != nil {
				log.Println("Websocket client going away:", err)
				c.Close(websocket.StatusAbnormalClosure, "Failed to read message")
				return
			}

			var msg WSMessage
			err = json.Unmarshal(buf, &msg)
			if err != nil {
				log.Println("Failed to parse websocket message:", err)
				c.Close(websocket.StatusAbnormalClosure, "Failed to parse message")
				return
			}

			err = handleWebsocketMessage(&msg)
			if err != nil {
				log.Println("Failed to handle websocket message:", err)
				c.Close(websocket.StatusAbnormalClosure, "Failed to handle message")
				return
			}
		}
	})

	http.HandleFunc("/api/remote/screen-size", handler(func(w RW, req *Req) error {
		if req.Method == "GET" {
			var size ScreenSizeData
			size.Width, size.Height = robotgo.GetScreenSize()
			return json.NewEncoder(w).Encode(&size)
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/remote/screencast", handler(func(w RW, req *Req) error {
		if req.Method == "GET" {
			w.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary=MEDIATOR_FRAME_BOUNDARY")
			w.WriteHeader(200)

			for {
				img := <-screencap.Capture()

				var err error
				_, err = w.Write([]byte(fmt.Sprintf(
					"--MEDIATOR_FRAME_BOUNDARY\r\n" +
					"Content-Type: image/jpeg\r\n" +
					"Content-Length: %d\r\n" +
					"\r\n", img.Length)))
				if errors.Is(err, syscall.EPIPE) {
					return nil
				} else if err != nil {
					log.Printf("Screencast: Header write error: %v", err)
					return nil
				}

				_, err = w.Write(img.Data[0:img.Length])
				if errors.Is(err, syscall.EPIPE) {
					return nil;
				} else if err != nil {
					log.Printf("Screencast: Body write error: %v", err)
					return nil
				}

				_, err = w.Write([]byte("\r\n"))
				if errors.Is(err, syscall.EPIPE) {
					return nil;
				} else if err != nil {
					log.Printf("Write error: %v", err)
					return nil
				}
			}
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}));

	http.HandleFunc("/api/dir/", handler(func(w RW, req *Req) error {
		if req.Method == "GET" {
			subPath := req.URL.Path[len("/api/dir/"):]
			dirEnts, err := ioutil.ReadDir(path.Join(conf.BasePath, subPath))
			if err != nil {
				return err
			}

			list := ListDirData{
				Entries: make([]DirEntryData, 0, len(dirEnts)),
			}

			for _, ent := range dirEnts {
				entType := "f"
				if ent.IsDir() {
					entType = "d"
				}

				list.Entries = append(list.Entries, DirEntryData{
					Name: ent.Name(),
					Type: entType,
				})
			}

			return json.NewEncoder(w).Encode(&list)
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/shutdown", handler(func(w RW, req *Req) error {
		if req.Method == "POST" {
			return platform.Shutdown()
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	go screencap.Run()

	log.Println("Listening on :3000...")
	err = http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
