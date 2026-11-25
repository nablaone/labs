# heartbeat_task.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

42007880 <led_set>:
# #include "heartbeat_task.h"
# 
# #define HB_PERIOD_MS 500
# 
# static void led_set(bool on)
# {
42007880:	1141                	add	sp,sp,-16
42007882:	c606                	sw	ra,12(sp)
# 	gpio_set_level(LED_GPIO, on ? LED_ON_LEVEL : !LED_ON_LEVEL);
42007884:	00154593          	xor	a1,a0,1
42007888:	4521                	li	a0,8
4200788a:	489190ef          	jal	42021512 <gpio_set_level>
# }
4200788e:	40b2                	lw	ra,12(sp)
42007890:	0141                	add	sp,sp,16
42007892:	8082                	ret

42007894 <heartbeat_task_init>:
# 
# void heartbeat_task_init(void)
# {
42007894:	1141                	add	sp,sp,-16
42007896:	c606                	sw	ra,12(sp)
# 	gpio_reset_pin(LED_GPIO);
42007898:	4521                	li	a0,8
4200789a:	75f190ef          	jal	420217f8 <gpio_reset_pin>
# 	gpio_set_direction(LED_GPIO, GPIO_MODE_OUTPUT);
4200789e:	4589                	li	a1,2
420078a0:	4521                	li	a0,8
420078a2:	4f9190ef          	jal	4202159a <gpio_set_direction>
# 	led_set(false);
420078a6:	4501                	li	a0,0
420078a8:	fd9ff0ef          	jal	42007880 <led_set>
# }
420078ac:	40b2                	lw	ra,12(sp)
420078ae:	0141                	add	sp,sp,16
420078b0:	8082                	ret

420078b2 <heartbeat_task>:
# 
# /* Toggles the LED every HB_PERIOD_MS -- a visible "firmware is alive"
#  * pulse, and the simplest example of the init/task module shape. */
# void heartbeat_task(void *arg)
# {
420078b2:	1141                	add	sp,sp,-16
420078b4:	c606                	sw	ra,12(sp)
420078b6:	c422                	sw	s0,8(sp)
# 	bool led_on = false;
420078b8:	4401                	li	s0,0
# 
# 	while (1) {
# 		vTaskDelay(pdMS_TO_TICKS(HB_PERIOD_MS));
420078ba:	03200513          	li	a0,50
420078be:	fe387097          	auipc	ra,0xfe387
420078c2:	320080e7          	jalr	800(ra) # 4038ebde <vTaskDelay>
# 		led_on = !led_on;
420078c6:	00144413          	xor	s0,s0,1
# 		led_set(led_on);
420078ca:	8522                	mv	a0,s0
420078cc:	fb5ff0ef          	jal	42007880 <led_set>
# 	while (1) {
420078d0:	b7ed                	j	420078ba <heartbeat_task+0x8>

