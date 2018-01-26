

#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>

#define LED1 PB3
#define LED2 PB2



ISR(TIMER1_OVF_vect)
{
    PORTB ^= _BV(LED1) | _BV(LED2);
}

void delay_ms(uint8_t ms) {
    uint16_t delay_count = 1000000 / 17500;
    volatile uint16_t i;
    
    while (ms != 0) {
        for (i=0; i != delay_count; i++);
        ms--;
    }
}

int main(void) {
    
    cli();
    
    // clk/8 
    TCCR1B |= (1 << CS11);
  
    // initialize counter
    TCNT1 = 0;
  
    // enable overflow interrupt
    TIMSK |= (1 << TOIE1);

    sei(); 
    
    DDRB = _BV(LED1) | _BV(LED2);
    // 
    PORTB = _BV(LED2);
    
    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
