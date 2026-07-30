SDK_PATH  := /Users/rafal/Library/Application Support/Garmin/ConnectIQ/Sdks/connectiq-sdk-mac-9.1.0-2026-03-09-6a872a80b
MONKEYC   := $(SDK_PATH)/bin/monkeyc
MONKEYDO  := $(SDK_PATH)/bin/monkeydo

DEVICE    := instincte45mm
KEY       := /Users/rafal/Documents/garmin/developer_key
JUNGLE    := monkey.jungle
OUTPUT    := build/helloworld.prg

WATCH_VOL := $(firstword $(wildcard /Volumes/INSTINCT*))

.PHONY: all build clean deploy deploy-watch keygen

all: build

## Build the .prg for the target device
build: $(OUTPUT)

$(OUTPUT): $(shell find source resources -type f) manifest.xml monkey.jungle
	@mkdir -p build
	"$(MONKEYC)" -o $(OUTPUT) -f $(JUNGLE) -y $(KEY) -d $(DEVICE) -w

## Remove build artifacts
clean:
	rm -rf build/

## Launch in the ConnectIQ simulator (runs build first)
deploy: build
	"$(MONKEYDO)" $(OUTPUT) $(DEVICE)

## Copy .prg to a USB-connected watch (mount it first)
deploy-watch: build
	@test -n "$(WATCH_VOL)" || (echo "ERROR: Watch not found under /Volumes/INSTINCT*"; exit 1)
	cp $(OUTPUT) "$(WATCH_VOL)/GARMIN/APPS/"
	diskutil eject "$(WATCH_VOL)"
	@echo "Deployed to $(WATCH_VOL)"

## Generate a self-signed developer key (one-time setup)
keygen:
	openssl genrsa -out developer_key.pem 4096
	openssl pkcs8 -topk8 -inform PEM -outform DER -in developer_key.pem -out $(KEY) -nocrypt
	@echo "Key written to $(KEY)"
