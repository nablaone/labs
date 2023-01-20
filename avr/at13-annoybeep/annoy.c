
#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>
#include <stdlib.h>
#include <util/delay.h>


volatile int counter = 0;
ISR(TIM0_COMPA_vect)
{
    if (counter < 0) {
        counter = 60 * 16 * (10 + rand() % 10);  //every 10-20 minutes
        PORTB = _BV(PB3);
    } else {
        counter--;
        PORTB = 0; 
    } 
}

int main(void) {
    
    cli();

    DDRB |= _BV(DDB3);
  

    TCCR0A |= (1 << COM0A1);  // clear timer on compare match
    TCCR0A |= _BV(WGM01); // Table  11-8. CTC

    TCCR0B |= (1 << CS02) | (1 << CS00); // clock by 1024      
    OCR0A = 64; // interrupt every 1/16s
    
    // enable interrupt
    TIMSK0 |= (1 << OCIE0A);
    sei();

    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
