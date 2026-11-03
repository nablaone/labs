package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

func getPatch(blo *Blofeld) {

	var Bank = "H"
	var Program = 128

	p, devID, err := blo.RequestPatchDump(Bank, Program)
	if err != nil {
		log.Fatalf("failed to read patch: %v", err)
	}
	log.Println("Patch name", p.Name)
	log.Printf("Read patch from Bank %s, Program %d (device 0x%02X): %+v\n", Bank, Program, devID, p)

	asJson, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal patch to JSON: %v", err)
	}

	fmt.Println(string(asJson))
}

func setPatch(blo *Blofeld) {

	patch := &Patch{}

	asJson, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("failed to read patch JSON from stdin: %v", err)
	}

	if err := json.Unmarshal(asJson, patch); err != nil {
		log.Fatalf("failed to unmarshal patch JSON: %v", err)
	}

	var Bank = "H"
	var Program = 128

	if err := blo.SendPatch(Bank, Program, patch); err != nil {
		log.Fatalf("failed to send patch: %v", err)
	}
}
