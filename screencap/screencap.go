package screencap

import "github.com/kbinani/screenshot"
import "image/jpeg"
import "sync"
import "time"
import "log"

type Buffer struct {
	Data []byte
	Length int
}

func (buf *Buffer) Write(data []byte) (int, error) {
	if len(buf.Data) == 0 {
		l := len(data)
		if l < 1024 {
			l = 2024
		}
		buf.Data = make([]byte, l)
	} else if buf.Length + len(data) > len(buf.Data) {
		newSize := len(buf.Data) * 2
		for buf.Length + len(data) >  newSize {
			newSize *= 2
		}

		newBuf := make([]byte, newSize)
		copy(newBuf, buf.Data[0:buf.Length])
		buf.Data = newBuf
	}

	copy(buf.Data[buf.Length:], data)
	buf.Length += len(data)
	return len(data), nil
}

var (
	mut = sync.Mutex{}
	chans = make([]chan *Buffer, 0)
	startChan = make(chan struct{}, 1)

	buffers = make([]Buffer, 4)
	currentBuffer = 0
)

func Capture() chan *Buffer {
	ch := make(chan *Buffer)
	mut.Lock()
	chans = append(chans, ch)
	mut.Unlock()

	select {
	case startChan <- struct{}{}:
	default:
	}

	return ch
}

func Run() {
	for {
		<-startChan

		img, err := screenshot.CaptureDisplay(0)
		if err != nil {
			log.Printf("Failed to capture screenshot: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		buf := &buffers[currentBuffer]
		currentBuffer = (currentBuffer + 1) % len(buffers)
		buf.Length = 0
		err = jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
		if err != nil {
			log.Printf("Failed to encode jpeg: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		mut.Lock()
		for _, ch := range chans {
			ch <- buf
		}
		chans = make([]chan *Buffer, 0)
		mut.Unlock()
	}
}
