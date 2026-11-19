NAME    := BRUT_ST
THEME   := dist/$(NAME).ReaperThemeZip
VENV    := .venv
PY      := $(VENV)/bin/python

UNAME := $(shell uname -s)
ifeq ($(UNAME),Darwin)
  REAPER_DIR ?= $(HOME)/Library/Application Support/REAPER
else ifneq (,$(findstring MINGW,$(UNAME))$(findstring MSYS,$(UNAME))$(findstring CYGWIN,$(UNAME)))
  REAPER_DIR ?= $(APPDATA)/REAPER
else
  REAPER_DIR ?= $(HOME)/.config/REAPER
endif
THEME_DIR := $(REAPER_DIR)/ColorThemes

.PHONY: all build install uninstall clean

all: build

$(PY):
	python3 -m venv $(VENV)
	$(PY) -m pip -q install pillow numpy

build: $(THEME)

$(THEME): tools/build.py | $(PY)
	$(PY) -I tools/build.py

install: $(THEME)
	@test -d "$(REAPER_DIR)" || { echo "REAPER resource folder not found: $(REAPER_DIR)"; echo "Override with: make install REAPER_DIR=/path/to/REAPER"; exit 1; }
	mkdir -p "$(THEME_DIR)"
	cp "$(THEME)" "$(THEME_DIR)/"
	@echo "Installed to $(THEME_DIR)"
	@echo "In REAPER: Options > Themes > $(NAME)"

uninstall:
	rm -f "$(THEME_DIR)/$(NAME).ReaperThemeZip"
	@echo "Removed. In REAPER pick another theme: Options > Themes > Default_7.0"

clean:
	rm -rf build dist
