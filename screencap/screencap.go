package screencap

import (
	"image/jpeg"
	"sync"
	"time"
	"log"

	"github.com/go-vgo/robotgo"
)

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
	targetDelta := 66 * time.Millisecond

	for {
		<-startChan

		startTime := time.Now()
		img := robotgo.CaptureImg()

		buf := &buffers[currentBuffer]
		currentBuffer = (currentBuffer + 1) % len(buffers)
		buf.Length = 0
		err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 40})
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

		delta := time.Now().Sub(startTime)
		if delta < targetDelta {
			time.Sleep(targetDelta - delta)
		}
	}
}
