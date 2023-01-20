

#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>
#include <util/delay.h>

ISR(TIMER1_COMPA_vect)
{
     PORTB ^= _BV(PB3);    
}

int main(void) {
    
    cli();
    TCCR1 |= (1 << CTC1);  // clear timer on compare match
    TCCR1 |= (1 << CS13) | (1 << CS12) | (1 << CS11); // by 8192
    // cpu_f 1000000 / 8192 / 120 ~= 1s 
    OCR1C = 120;
    TIMSK |= (1 << OCIE1A);
    sei();
    DDRB |= _BV(DDB3);
    
    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
