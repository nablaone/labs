

#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>
#include <util/delay.h>

volatile int counter = 0;

ISR(TIMER1_COMPA_vect)
{
    if (counter == 0 || counter == 5) {
        PORTB ^= _BV(PB3); 
    }

    if(counter == 0 || counter == 5) {
        PORTB |= _BV(PB4);
    }

    if(counter == 1 || counter == 6) {
        PORTB ^= _BV(PB4);
    }

    if(counter == 11) {
        counter = 0;
    } else {
        counter++;
    }
    //PORTB ^= _BV(PB4);   
}

int main(void) {
    
    cli();
    TCCR1 |= (1 << CTC1);  // clear timer on compare match
    TCCR1 |= (1 << CS13) | (1 << CS12) | (1 << CS11); // by 8192
    // cpu_f 1000000 / 8192 / 120 ~= 1s 
    OCR1C = 12;
    TIMSK |= (1 << OCIE1A);
    
    DDRB |= _BV(DDB3);
    DDRB |= _BV(DDB4);
    
    sei();
    

    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
