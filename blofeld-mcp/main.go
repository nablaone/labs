package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gitlab.com/gomidi/midi/v2"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

func main() {
	const ()

	log.Println("Available MIDI outputs:")
	log.Print(midi.GetOutPorts().String())

	blo := NewBlofeld()

	defer blo.Close()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "play":
			playTestNotes(blo)
			return
		case "single":
			singleTest(blo)
			return
		case "get":
			getPatch(blo)
			return
		case "set":
			setPatch(blo)
			return

		case "mcp":
			runMCP(blo)
			return

		default:
			log.Fatalf("unknown command %q", os.Args[1])
		}
	}
	log.Println("exiting: no command specified")
}

func bankToByte(bank string) (byte, error) {
	if bank == "" {
		return 0, errors.New("bank must not be empty")
	}
	ch := strings.ToUpper(bank)[0]
	if ch < 'A' || ch > 'H' {
		return 0, fmt.Errorf("bank must be A–H, got %q", bank)
	}
	return byte(ch - 'A'), nil
}
