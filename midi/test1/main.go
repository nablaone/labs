package main

import (
	"log"
	"time"

	"github.com/rakyll/portmidi"
)

func main() {
	portmidi.Initialize()
	defer portmidi.Terminate()

	// List available devices
	for i := 0; i < portmidi.CountDevices(); i++ {
		info := portmidi.Info(portmidi.DeviceID(i))
		if info.IsOutputAvailable {
			log.Printf("[%d] %s\n", i, info.Name)
		}
	}

	// Open a device (e.g. IAC Driver or external synth)
	// Change this index to match your system's virtual port
	out, err := portmidi.NewOutputStream(portmidi.DeviceID(3), 1024, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	scale := []int{60, 62, 64, 65, 67, 69, 71, 72}
	pattern := []int{1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 0, 1, 0, 0}

	bpm := 120
	stepDuration := time.Duration(60000/bpm/4) * time.Millisecond // 16th notes

	log.Println("🔥 Playing live... Press Ctrl+C to stop.")

	for step := 0; ; step++ {
		idx := step % len(pattern)
		if pattern[idx] == 1 {
			note := scale[step%len(scale)]
			out.WriteShort(0x90, int64(note), 100) // Note On
			go func(n int64) {
				time.Sleep(stepDuration / 2)
				out.WriteShort(0x80, n, 0) // Note Off
			}(int64(note))
		}
		time.Sleep(stepDuration)
	}
}
