

#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>
#include <util/delay.h>


ISR(TIM0_COMPA_vect)
{
     PORTB ^= _BV(PB3); 
}


int main(void) {
    
    cli();

    TCCR0A |= (1 << COM0A1);  // clear timer on compare match
    TCCR0A |= _BV(WGM01); // Table  11-8. CTC

    TCCR0B |= (1 << CS02) | (1 << CS00); // clock by 1024      
    OCR0A = 255;


    // enable interrupt
    TIMSK0 |= (1 << OCIE0A);
    sei();
    
    DDRB |= _BV(DDB3);
    
    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
