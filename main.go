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
			return nil
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
				log.Printf("Got image, %v bytes", img.Length)

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
	err = http.ListenAndServe("localhost:3000", nil)
	if err != nil {
		log.Fatal(err)
	}
}
