

#include <avr/io.h>
#include <avr/interrupt.h>
#include <avr/sleep.h>

#define LED1 PB3
#define LED2 PB2

#define SPEAK PB1


int speak = 0;

noise(int onOff) {
    if (onOff) {
        speak = 1;
    } else {
        int mask = 0xff ^ (1 << SPEAK);
        PORTB = mask & PORTB;
        speak = 0;
    } 
}

int toggle = 0;

ISR(TIMER1_OVF_vect)
{
    toggle = 1 - toggle;
    PORTB ^= _BV(LED1) | _BV(LED2);
    
    noise(toggle);

    
}

int count=0;


ISR(TIMER0_OVF_vect)
{

    if(speak) {
        count++;
        if (count > 3) {
           PORTB ^= _BV(SPEAK);
           count=0;
        }
    }
}



int main(void) {
    
    cli();
    
    // timer1
    // clk/8 
    TCCR1B |= (1 << CS11);
  
    // initialize counter
    TCNT1 = 0;
  
    // enable overflow interrupt
    TIMSK |= (1 << TOIE1);

    // timer0 
    //  
    TCCR0B |=  (1 << CS01);
  
    // initialize counter
    TCNT0 = 0;
  
    // enable overflow interrupt
    TIMSK |= (1 << TOIE0);


    sei(); 
    
    DDRB = _BV(LED1) | _BV(LED2) | _BV(SPEAK);
    // 
    PORTB = _BV(LED2);
    
    for(;;) {
        sleep_cpu();
    }
    
    return 0;
}
