package main

import "errors"
import "log"
import "os"
import "fmt"
import "path"
import "net/http"
import "io/ioutil"
import "encoding/json"
import "github.com/go-vgo/robotgo"
import "github.com/BurntSushi/toml"
import "coffee.mort.mediator/screencap"

type Config struct {
	BasePath string `toml:"base_path"`
}

type EmptyData struct {}

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

	http.HandleFunc("/api/remote/screen-size", handler(func(w RW, req *Req) error {
		if req.Method == "GET" {
			var size ScreenSizeData
			size.Width, size.Height = robotgo.GetScreenSize()
			return json.NewEncoder(w).Encode(&size)
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/remote/mouse-pos", handler(func(w RW, req *Req) error {
		if req.Method == "GET" {
			var pos MousePosData
			pos.X, pos.Y = robotgo.GetMousePos()
			return json.NewEncoder(w).Encode(&pos)
		} else if req.Method == "PUT" {
			var pos MousePosData
			err := json.NewDecoder(req.Body).Decode(&pos)
			if err != nil {
				return err
			}

			robotgo.MoveMouse(pos.X, pos.Y)
			return json.NewEncoder(w).Encode(&EmptyData{})
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/remote/mouse-click", handler(func(w RW, req *Req) error {
		if req.Method == "POST" {
			var click MouseClickData
			err := json.NewDecoder(req.Body).Decode(&click)
			if err != nil {
				return err
			}

			robotgo.MouseClick(click.Button, click.DoubleClick)
			return json.NewEncoder(w).Encode(&EmptyData{})
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/remote/scroll", handler(func(w RW, req *Req) error {
		if req.Method == "POST" {
			var scroll ScrollData
			err := json.NewDecoder(req.Body).Decode(&scroll)
			if err != nil {
				return err
			}

			robotgo.Scroll(scroll.X, scroll.Y)
			return json.NewEncoder(w).Encode(&EmptyData{})
		} else {
			return errors.New("Invalid method: "+ req.Method)
		}
	}))

	http.HandleFunc("/api/remote/keyboard-type", handler(func(w RW, req *Req) error {
		if req.Method == "POST" {
			var text KeyboardTypeData
			err := json.NewDecoder(req.Body).Decode(&text)
			if err != nil {
				return err
			}

			robotgo.TypeStr(text.Text)
			return json.NewEncoder(w).Encode(&EmptyData{})
		} else {
			return errors.New("Invalid method: " + req.Method)
		}
	}))

	http.HandleFunc("/api/remote/keyboard-key", handler(func(w RW, req *Req) error {
		if req.Method == "POST" {
			var key KeyboardKeyData
			err := json.NewDecoder(req.Body).Decode(&key)
			if err != nil {
				return err
			}

			var modifiers []interface{}
			for _, modifier := range key.Modifiers {
				modifiers = append(modifiers, modifier)
			}

			log.Printf("key: %s, modifiers: %#v", key.Key, modifiers)
			robotgo.KeyTap(key.Key, modifiers...)
			return json.NewEncoder(w).Encode(&EmptyData{})
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
				if err != nil {
					log.Printf("Write error: %v", err)
					return nil
				}

				_, err = w.Write(img.Data[0:img.Length])
				if err != nil {
					log.Printf("Write error: %v", err)
					return nil
				}

				_, err = w.Write([]byte("\r\n"))
				if err != nil {
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

	go screencap.Run()

	log.Println("Listening on :3000...")
	err = http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
