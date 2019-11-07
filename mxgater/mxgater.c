

#include <avr/io.h>
#include <util/delay.h>
#include <stdlib.h>



#define SWITCH PB4
#define LED PB3
#define MOTOR PB1

int main(void) {
    
    DDRB |= _BV(LED);
    DDRB |= _BV(MOTOR);
    
    DDRB &= ~(1 << SWITCH); // input
    PORTB |= 1 << SWITCH; // pull up
    
    for(;;) {
        
        PORTB |= _BV(MOTOR);


        // wait for button
        int count=0; 

        while(PINB & _BV(SWITCH)) {
            _delay_ms(100);
            count++;

            if(count > 5) {
                PORTB ^= _BV(LED);
                count = 0;
            }   
        }

        while(!(PINB & _BV(SWITCH))) {
            _delay_ms(50);
            PORTB ^= _BV(LED);
        }

        PORTB |= _BV(LED);

        // random delay 

        int delay = (5 + 5 % rand());
        while(delay > 0) {
            _delay_ms(1000);
            delay--;
        }

        PORTB ^= _BV(LED);
        PORTB ^= _BV(MOTOR);
        
        _delay_ms(1000);
        PORTB |= _BV(MOTOR);
        
    }
    
    return 0;
}
