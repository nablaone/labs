
ARCH=attiny13
CC_ARGS=-DF_CPU=1048576
AVR_DUDE_ARGS=-c usbasp -p t13 -B10
APP=blink

all: $(APP).hex

%.o: %.c
	avr-gcc $(CC_ARGS) -g -Os -mmcu=$(ARCH) -c $< -o $@

%.elf: %.o
	avr-gcc -g -mmcu=$(ARCH) -o $@ $<

%.hex: %.elf
	avr-objcopy -j .text -j .data -O ihex $< $@

%.lst: %.elf
	avr-objdump -h -S $< > $@

clean:
	rm -f $(APP).o $(APP).elf $(APP).hex $(APP).lst

.PHONY: clean


flash: $(APP).hex
	avrdude $(AVR_DUDE_ARGS) -U flash:w:$< 

.PHONY: flash

avrclean:
	avrdude $(AVR_DUDE_ARGS) -e 

avrstatus:
	avrdude $(AVR_DUDE_ARGS) -U hfuse:r:-:h -U lfuse:r:-:h

.PHONY: avrclean avrstatus 



