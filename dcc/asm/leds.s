# leds.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

40381e50 <probe_done>:
# static rmt_symbol_word_t probe_buf[PROBE_BITS + 16];
# 
# /* RMT ISR context: hand the symbol count to the waiting CLI task. */
# static bool IRAM_ATTR probe_done(rmt_channel_handle_t ch,
# 				 const rmt_rx_done_event_data_t *ev, void *arg)
# {
40381e50:	1101                	add	sp,sp,-32
40381e52:	ce06                	sw	ra,28(sp)
40381e54:	8532                	mv	a0,a2
# 	BaseType_t woken = pdFALSE;
40381e56:	c602                	sw	zero,12(sp)
# 	size_t n = ev->num_symbols;
40381e58:	41dc                	lw	a5,4(a1)
40381e5a:	c43e                	sw	a5,8(sp)
# 	xQueueSendFromISR((QueueHandle_t)arg, &n, &woken);
40381e5c:	4681                	li	a3,0
40381e5e:	0070                	add	a2,sp,12
40381e60:	002c                	add	a1,sp,8
40381e62:	3f60b0ef          	jal	4038d258 <xQueueGenericSendFromISR>
# 	return woken == pdTRUE;
40381e66:	4532                	lw	a0,12(sp)
40381e68:	157d                	add	a0,a0,-1
# }
40381e6a:	00153513          	seqz	a0,a0
40381e6e:	40f2                	lw	ra,28(sp)
40381e70:	6105                	add	sp,sp,32
40381e72:	8082                	ret

4200900e <build_frame>:
# 	for (int i = 0; i < LEDS_COUNT; i++) {
4200900e:	4601                	li	a2,0
42009010:	a80d                	j	42009042 <build_frame+0x34>
# 		uint8_t r = 0, g = 0, b = 0;
42009012:	4701                	li	a4,0
42009014:	4581                	li	a1,0
42009016:	4681                	li	a3,0
# 		uint8_t *p = frame + i * 3;
42009018:	00161793          	sll	a5,a2,0x1
4200901c:	97b2                	add	a5,a5,a2
4200901e:	97aa                	add	a5,a5,a0
# 		p[0] = order_rgb ? r : g;
42009020:	3fc99837          	lui	a6,0x3fc99
42009024:	38884803          	lbu	a6,904(a6) # 3fc99388 <order_rgb>
42009028:	08080463          	beqz	a6,420090b0 <build_frame+0xa2>
4200902c:	88b6                	mv	a7,a3
4200902e:	01178023          	sb	a7,0(a5)
# 		p[1] = order_rgb ? g : r;
42009032:	00080363          	beqz	a6,42009038 <build_frame+0x2a>
42009036:	86ae                	mv	a3,a1
42009038:	00d780a3          	sb	a3,1(a5)
# 		p[2] = b;
4200903c:	00e78123          	sb	a4,2(a5)
# 	for (int i = 0; i < LEDS_COUNT; i++) {
42009040:	0605                	add	a2,a2,1
42009042:	479d                	li	a5,7
42009044:	06c7c863          	blt	a5,a2,420090b4 <build_frame+0xa6>
# 		if (leds[i].on) {
42009048:	3fc997b7          	lui	a5,0x3fc99
4200904c:	00261713          	sll	a4,a2,0x2
42009050:	e0c78793          	add	a5,a5,-500 # 3fc98e0c <leds>
42009054:	97ba                	add	a5,a5,a4
42009056:	0007c783          	lbu	a5,0(a5)
4200905a:	dfc5                	beqz	a5,42009012 <build_frame+0x4>
# 			r = (leds[i].r * brightness + 127) / 255;
4200905c:	3fc99737          	lui	a4,0x3fc99
42009060:	00261793          	sll	a5,a2,0x2
42009064:	e0c70713          	add	a4,a4,-500 # 3fc98e0c <leds>
42009068:	973e                	add	a4,a4,a5
4200906a:	00174683          	lbu	a3,1(a4)
4200906e:	8501c783          	lbu	a5,-1968(gp) # 3fc95250 <brightness>
42009072:	02f686b3          	mul	a3,a3,a5
42009076:	07f68693          	add	a3,a3,127
4200907a:	0ff00813          	li	a6,255
4200907e:	0306c6b3          	div	a3,a3,a6
42009082:	0ff6f693          	zext.b	a3,a3
# 			g = (leds[i].g * brightness + 127) / 255;
42009086:	00274583          	lbu	a1,2(a4)
4200908a:	02b785b3          	mul	a1,a5,a1
4200908e:	07f58593          	add	a1,a1,127
42009092:	0305c5b3          	div	a1,a1,a6
42009096:	0ff5f593          	zext.b	a1,a1
# 			b = (leds[i].b * brightness + 127) / 255;
4200909a:	00374703          	lbu	a4,3(a4)
4200909e:	02e78733          	mul	a4,a5,a4
420090a2:	07f70713          	add	a4,a4,127
420090a6:	03074733          	div	a4,a4,a6
420090aa:	0ff77713          	zext.b	a4,a4
420090ae:	b7ad                	j	42009018 <build_frame+0xa>
# 		p[0] = order_rgb ? r : g;
420090b0:	88ae                	mv	a7,a1
420090b2:	bfb5                	j	4200902e <build_frame+0x20>
# }
420090b4:	8082                	ret

420090b6 <changed>:
# {
420090b6:	1141                	add	sp,sp,-16
420090b8:	c606                	sw	ra,12(sp)
# 	if (change_cb) {
420090ba:	3fc997b7          	lui	a5,0x3fc99
420090be:	3847a783          	lw	a5,900(a5) # 3fc99384 <change_cb>
420090c2:	c391                	beqz	a5,420090c6 <changed+0x10>
# 		change_cb();
420090c4:	9782                	jalr	a5
# }
420090c6:	40b2                	lw	ra,12(sp)
420090c8:	0141                	add	sp,sp,16
420090ca:	8082                	ret

420090cc <send_frame_locked>:
# {
420090cc:	1101                	add	sp,sp,-32
420090ce:	ce06                	sw	ra,28(sp)
420090d0:	cc22                	sw	s0,24(sp)
420090d2:	ca26                	sw	s1,20(sp)
420090d4:	862a                	mv	a2,a0
420090d6:	86ae                	mv	a3,a1
# 	rmt_transmit_config_t tx = { .loop_count = 0 };
420090d8:	c402                	sw	zero,8(sp)
420090da:	c602                	sw	zero,12(sp)
# 	esp_err_t err = rmt_transmit(chan, encoder, frame, len, &tx);
420090dc:	0038                	add	a4,sp,8
420090de:	3fc997b7          	lui	a5,0x3fc99
420090e2:	3907a583          	lw	a1,912(a5) # 3fc99390 <encoder>
420090e6:	3fc997b7          	lui	a5,0x3fc99
420090ea:	3947a503          	lw	a0,916(a5) # 3fc99394 <chan>
420090ee:	4cc1e0ef          	jal	420275ba <rmt_transmit>
420090f2:	842a                	mv	s0,a0
# 	if (err == ESP_OK) {
420090f4:	e11d                	bnez	a0,4200911a <send_frame_locked+0x4e>
# 		rmt_tx_wait_all_done(chan, portMAX_DELAY);
420090f6:	55fd                	li	a1,-1
420090f8:	3fc997b7          	lui	a5,0x3fc99
420090fc:	3947a503          	lw	a0,916(a5) # 3fc99394 <chan>
42009100:	6e41e0ef          	jal	420277e4 <rmt_tx_wait_all_done>
# 		esp_rom_delay_us(RESET_US);
42009104:	12c00513          	li	a0,300
42009108:	fdff7097          	auipc	ra,0xfdff7
4200910c:	f48080e7          	jalr	-184(ra) # 40000050 <esp_rom_delay_us>
# }
42009110:	40f2                	lw	ra,28(sp)
42009112:	4462                	lw	s0,24(sp)
42009114:	44d2                	lw	s1,20(sp)
42009116:	6105                	add	sp,sp,32
42009118:	8082                	ret
# 		ESP_LOGE(TAG, "transmit: %s", esp_err_to_name(err));
4200911a:	fe38a097          	auipc	ra,0xfe38a
4200911e:	5c2080e7          	jalr	1474(ra) # 403936dc <esp_log_timestamp>
42009122:	84aa                	mv	s1,a0
42009124:	8522                	mv	a0,s0
42009126:	f37f80ef          	jal	4200205c <esp_err_to_name>
4200912a:	87aa                	mv	a5,a0
4200912c:	3c0755b7          	lui	a1,0x3c075
42009130:	0f458713          	add	a4,a1,244 # 3c0750f4 <_flash_rodata_start+0x4fd4>
42009134:	86a6                	mv	a3,s1
42009136:	3c075637          	lui	a2,0x3c075
4200913a:	0fc60613          	add	a2,a2,252 # 3c0750fc <_flash_rodata_start+0x4fdc>
4200913e:	0f458593          	add	a1,a1,244
42009142:	4505                	li	a0,1
42009144:	fe38a097          	auipc	ra,0xfe38a
42009148:	4aa080e7          	jalr	1194(ra) # 403935ee <esp_log_write>
# }
4200914c:	b7d1                	j	42009110 <send_frame_locked+0x44>

4200914e <show_locked>:
# {
4200914e:	7179                	add	sp,sp,-48
42009150:	d606                	sw	ra,44(sp)
# 	build_frame(frame);
42009152:	0028                	add	a0,sp,8
42009154:	ebbff0ef          	jal	4200900e <build_frame>
# 	send_frame_locked(frame, sizeof(frame));
42009158:	45e1                	li	a1,24
4200915a:	0028                	add	a0,sp,8
4200915c:	f71ff0ef          	jal	420090cc <send_frame_locked>
# }
42009160:	50b2                	lw	ra,44(sp)
42009162:	6145                	add	sp,sp,48
42009164:	8082                	ret

42009166 <usage>:
# 	}
# 	return 1;
# }
# 
# static int usage(void)
# {
42009166:	1141                	add	sp,sp,-16
42009168:	c606                	sw	ra,12(sp)
# 	printf("usage:\n"
4200916a:	4619                	li	a2,6
4200916c:	4595                	li	a1,5
4200916e:	3c075537          	lui	a0,0x3c075
42009172:	12450513          	add	a0,a0,292 # 3c075124 <_flash_rodata_start+0x5004>
42009176:	58d510ef          	jal	4205af02 <printf>
# 	       "  led order grb|rgb         WS2812 = grb, PL9823 = rgb\n"
# 	       "  led test                  walk red/green/blue along the chain\n"
# 	       "  led probe                 loopback self-test, jumper GPIO%d -> GPIO%d\n",
# 	       LEDS_GPIO, LEDS_PROBE_GPIO);
# 	return 1;
# }
4200917a:	4505                	li	a0,1
4200917c:	40b2                	lw	ra,12(sp)
4200917e:	0141                	add	sp,sp,16
42009180:	8082                	ret

42009182 <probe_setup>:
# {
42009182:	7179                	add	sp,sp,-48
42009184:	d606                	sw	ra,44(sp)
# 	if (probe_chan) {
42009186:	3fc997b7          	lui	a5,0x3fc99
4200918a:	3807a783          	lw	a5,896(a5) # 3fc99380 <probe_chan>
4200918e:	c789                	beqz	a5,42009198 <probe_setup+0x16>
# 		return ESP_OK;
42009190:	4501                	li	a0,0
# }
42009192:	50b2                	lw	ra,44(sp)
42009194:	6145                	add	sp,sp,48
42009196:	8082                	ret
# 	rmt_rx_channel_config_t cfg = {
42009198:	c402                	sw	zero,8(sp)
4200919a:	c602                	sw	zero,12(sp)
4200919c:	c802                	sw	zero,16(sp)
4200919e:	ca02                	sw	zero,20(sp)
420091a0:	cc02                	sw	zero,24(sp)
420091a2:	ce02                	sw	zero,28(sp)
420091a4:	4799                	li	a5,6
420091a6:	c43e                	sw	a5,8(sp)
420091a8:	4791                	li	a5,4
420091aa:	c63e                	sw	a5,12(sp)
420091ac:	009897b7          	lui	a5,0x989
420091b0:	68078793          	add	a5,a5,1664 # 989680 <_rtc_fast_length+0x989480>
420091b4:	c83e                	sw	a5,16(sp)
420091b6:	03000793          	li	a5,48
420091ba:	ca3e                	sw	a5,20(sp)
# 	esp_err_t err = rmt_new_rx_channel(&cfg, &probe_chan);
420091bc:	3fc995b7          	lui	a1,0x3fc99
420091c0:	38058593          	add	a1,a1,896 # 3fc99380 <probe_chan>
420091c4:	0028                	add	a0,sp,8
420091c6:	7be1c0ef          	jal	42025984 <rmt_new_rx_channel>
# 	if (err != ESP_OK) {
420091ca:	c511                	beqz	a0,420091d6 <probe_setup+0x54>
# 		probe_chan = NULL;
420091cc:	3fc997b7          	lui	a5,0x3fc99
420091d0:	3807a023          	sw	zero,896(a5) # 3fc99380 <probe_chan>
# 		return err;
420091d4:	bf7d                	j	42009192 <probe_setup+0x10>
# 	gpio_pulldown_en(LEDS_PROBE_GPIO);
420091d6:	4519                	li	a0,6
420091d8:	775170ef          	jal	4202114c <gpio_pulldown_en>
# 	probe_queue = xQueueCreate(1, sizeof(size_t));
420091dc:	4601                	li	a2,0
420091de:	4591                	li	a1,4
420091e0:	4505                	li	a0,1
420091e2:	fe384097          	auipc	ra,0xfe384
420091e6:	cc8080e7          	jalr	-824(ra) # 4038ceaa <xQueueGenericCreate>
420091ea:	862a                	mv	a2,a0
420091ec:	3fc997b7          	lui	a5,0x3fc99
420091f0:	36a7ae23          	sw	a0,892(a5) # 3fc9937c <probe_queue>
# 	rmt_rx_event_callbacks_t cbs = { .on_recv_done = probe_done };
420091f4:	403827b7          	lui	a5,0x40382
420091f8:	e5078793          	add	a5,a5,-432 # 40381e50 <probe_done>
420091fc:	c23e                	sw	a5,4(sp)
# 	ESP_ERROR_CHECK(rmt_rx_register_event_callbacks(probe_chan, &cbs, probe_queue));
420091fe:	004c                	add	a1,sp,4
42009200:	3fc997b7          	lui	a5,0x3fc99
42009204:	3807a503          	lw	a0,896(a5) # 3fc99380 <probe_chan>
42009208:	6331c0ef          	jal	4202603a <rmt_rx_register_event_callbacks>
4200920c:	e901                	bnez	a0,4200921c <probe_setup+0x9a>
# 	return rmt_enable(probe_chan);
4200920e:	3fc997b7          	lui	a5,0x3fc99
42009212:	3807a503          	lw	a0,896(a5) # 3fc99380 <probe_chan>
42009216:	6ab1b0ef          	jal	420250c0 <rmt_enable>
4200921a:	bfa5                	j	42009192 <probe_setup+0x10>
# 	ESP_ERROR_CHECK(rmt_rx_register_event_callbacks(probe_chan, &cbs, probe_queue));
4200921c:	3c075737          	lui	a4,0x3c075
42009220:	2b070713          	add	a4,a4,688 # 3c0752b0 <_flash_rodata_start+0x5190>
42009224:	3c07e6b7          	lui	a3,0x3c07e
42009228:	c4468693          	add	a3,a3,-956 # 3c07dc44 <__func__.0>
4200922c:	15c00613          	li	a2,348
42009230:	3c0755b7          	lui	a1,0x3c075
42009234:	2f058593          	add	a1,a1,752 # 3c0752f0 <_flash_rodata_start+0x51d0>
42009238:	fe383097          	auipc	ra,0xfe383
4200923c:	7b6080e7          	jalr	1974(ra) # 4038c9ee <_esp_error_check_failed>

42009240 <print_hist>:
# {
42009240:	1141                	add	sp,sp,-16
42009242:	c606                	sw	ra,12(sp)
42009244:	c422                	sw	s0,8(sp)
42009246:	c226                	sw	s1,4(sp)
42009248:	c04a                	sw	s2,0(sp)
4200924a:	892e                	mv	s2,a1
4200924c:	84b2                	mv	s1,a2
# 	printf("  %s:", name);
4200924e:	85aa                	mv	a1,a0
42009250:	3c075537          	lui	a0,0x3c075
42009254:	30450513          	add	a0,a0,772 # 3c075304 <_flash_rodata_start+0x51e4>
42009258:	4ab510ef          	jal	4205af02 <printf>
# 	for (int t = 0; t < bins; t++) {
4200925c:	4401                	li	s0,0
4200925e:	a00d                	j	42009280 <print_hist+0x40>
# 			printf("  %s%d.%d us x%d", t == bins - 1 ? ">=" : "", t / 10, t % 10, hist[t]);
42009260:	3c0757b7          	lui	a5,0x3c075
42009264:	30078593          	add	a1,a5,768 # 3c075300 <_flash_rodata_start+0x51e0>
42009268:	4629                	li	a2,10
4200926a:	02c466b3          	rem	a3,s0,a2
4200926e:	02c44633          	div	a2,s0,a2
42009272:	3c075537          	lui	a0,0x3c075
42009276:	30c50513          	add	a0,a0,780 # 3c07530c <_flash_rodata_start+0x51ec>
4200927a:	489510ef          	jal	4205af02 <printf>
# 	for (int t = 0; t < bins; t++) {
4200927e:	0405                	add	s0,s0,1
42009280:	02945063          	bge	s0,s1,420092a0 <print_hist+0x60>
# 		if (hist[t]) {
42009284:	00241793          	sll	a5,s0,0x2
42009288:	97ca                	add	a5,a5,s2
4200928a:	4398                	lw	a4,0(a5)
4200928c:	db6d                	beqz	a4,4200927e <print_hist+0x3e>
# 			printf("  %s%d.%d us x%d", t == bins - 1 ? ">=" : "", t / 10, t % 10, hist[t]);
4200928e:	fff48793          	add	a5,s1,-1
42009292:	fc8787e3          	beq	a5,s0,42009260 <print_hist+0x20>
42009296:	3c0715b7          	lui	a1,0x3c071
4200929a:	cdc58593          	add	a1,a1,-804 # 3c070cdc <_flash_rodata_start+0xbbc>
4200929e:	b7e9                	j	42009268 <print_hist+0x28>
# 	printf("\n");
420092a0:	4529                	li	a0,10
420092a2:	54b510ef          	jal	4205afec <putchar>
# }
420092a6:	40b2                	lw	ra,12(sp)
420092a8:	4422                	lw	s0,8(sp)
420092aa:	4492                	lw	s1,4(sp)
420092ac:	4902                	lw	s2,0(sp)
420092ae:	0141                	add	sp,sp,16
420092b0:	8082                	ret

420092b2 <cmd_led_probe>:
# {
420092b2:	7115                	add	sp,sp,-224
420092b4:	cf86                	sw	ra,220(sp)
420092b6:	cda2                	sw	s0,216(sp)
420092b8:	cba6                	sw	s1,212(sp)
420092ba:	c9ca                	sw	s2,208(sp)
420092bc:	c7ce                	sw	s3,204(sp)
# 	esp_err_t err = probe_setup();
420092be:	ec5ff0ef          	jal	42009182 <probe_setup>
# 	if (err != ESP_OK) {
420092c2:	e119                	bnez	a0,420092c8 <cmd_led_probe+0x16>
# 	for (size_t i = 0; i < sizeof(frame); i++) {
420092c4:	4701                	li	a4,0
420092c6:	a82d                	j	42009300 <cmd_led_probe+0x4e>
# 		printf("probe: can't set up RMT receive on GPIO%d: %s\n",
420092c8:	d95f80ef          	jal	4200205c <esp_err_to_name>
420092cc:	862a                	mv	a2,a0
420092ce:	4599                	li	a1,6
420092d0:	3c075537          	lui	a0,0x3c075
420092d4:	32050513          	add	a0,a0,800 # 3c075320 <_flash_rodata_start+0x5200>
420092d8:	42b510ef          	jal	4205af02 <printf>
# 		return 1;
420092dc:	4485                	li	s1,1
420092de:	a201                	j	420093de <cmd_led_probe+0x12c>
# 		frame[i] = pat[i % 3];
420092e0:	478d                	li	a5,3
420092e2:	02f777b3          	remu	a5,a4,a5
420092e6:	3c07e6b7          	lui	a3,0x3c07e
420092ea:	c5068693          	add	a3,a3,-944 # 3c07dc50 <pat.1>
420092ee:	97b6                	add	a5,a5,a3
420092f0:	0007c683          	lbu	a3,0(a5)
420092f4:	0c070793          	add	a5,a4,192
420092f8:	978a                	add	a5,a5,sp
420092fa:	fed78423          	sb	a3,-24(a5)
# 	for (size_t i = 0; i < sizeof(frame); i++) {
420092fe:	0705                	add	a4,a4,1
42009300:	47dd                	li	a5,23
42009302:	fce7ffe3          	bgeu	a5,a4,420092e0 <cmd_led_probe+0x2e>
# 	rmt_receive_config_t rx = {
42009306:	cf02                	sw	zero,156(sp)
42009308:	d102                	sw	zero,160(sp)
4200930a:	d302                	sw	zero,164(sp)
4200930c:	06400793          	li	a5,100
42009310:	cf3e                	sw	a5,156(sp)
42009312:	6795                	lui	a5,0x5
42009314:	e2078793          	add	a5,a5,-480 # 4e20 <_rtc_fast_length+0x4c20>
42009318:	d13e                	sw	a5,160(sp)
# 	xQueueReset(probe_queue);
4200931a:	4581                	li	a1,0
4200931c:	3fc997b7          	lui	a5,0x3fc99
42009320:	37c7a503          	lw	a0,892(a5) # 3fc9937c <probe_queue>
42009324:	fe384097          	auipc	ra,0xfe384
42009328:	986080e7          	jalr	-1658(ra) # 4038ccaa <xQueueGenericReset>
# 	xSemaphoreTake(lock, portMAX_DELAY);
4200932c:	55fd                	li	a1,-1
4200932e:	3fc997b7          	lui	a5,0x3fc99
42009332:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
42009336:	fe384097          	auipc	ra,0xfe384
4200933a:	346080e7          	jalr	838(ra) # 4038d67c <xQueueSemaphoreTake>
# 	err = rmt_receive(probe_chan, probe_buf, sizeof(probe_buf), &rx);
4200933e:	0974                	add	a3,sp,156
42009340:	34000613          	li	a2,832
42009344:	3fc995b7          	lui	a1,0x3fc99
42009348:	acc58593          	add	a1,a1,-1332 # 3fc98acc <probe_buf>
4200934c:	3fc997b7          	lui	a5,0x3fc99
42009350:	3807a503          	lw	a0,896(a5) # 3fc99380 <probe_chan>
42009354:	5791c0ef          	jal	420260cc <rmt_receive>
42009358:	842a                	mv	s0,a0
# 	if (err == ESP_OK) {
4200935a:	c125                	beqz	a0,420093ba <cmd_led_probe+0x108>
# 	xSemaphoreGive(lock);
4200935c:	4681                	li	a3,0
4200935e:	4601                	li	a2,0
42009360:	4581                	li	a1,0
42009362:	3fc997b7          	lui	a5,0x3fc99
42009366:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
4200936a:	fe384097          	auipc	ra,0xfe384
4200936e:	c46080e7          	jalr	-954(ra) # 4038cfb0 <xQueueGenericSend>
# 	if (err != ESP_OK) {
42009372:	e839                	bnez	s0,420093c8 <cmd_led_probe+0x116>
# 	size_t n = 0;
42009374:	cd02                	sw	zero,152(sp)
# 	if (xQueueReceive(probe_queue, &n, pdMS_TO_TICKS(50)) != pdTRUE) {
42009376:	4615                	li	a2,5
42009378:	092c                	add	a1,sp,152
4200937a:	3fc997b7          	lui	a5,0x3fc99
4200937e:	37c7a503          	lw	a0,892(a5) # 3fc9937c <probe_queue>
42009382:	fe384097          	auipc	ra,0xfe384
42009386:	190080e7          	jalr	400(ra) # 4038d512 <xQueueReceive>
4200938a:	84aa                	mv	s1,a0
4200938c:	4785                	li	a5,1
4200938e:	06f51063          	bne	a0,a5,420093ee <cmd_led_probe+0x13c>
# 	int hi[BINS] = { 0 }, lo[BINS] = { 0 };
42009392:	04000613          	li	a2,64
42009396:	4581                	li	a1,0
42009398:	08a8                	add	a0,sp,88
4200939a:	fdff7097          	auipc	ra,0xfdff7
4200939e:	fba080e7          	jalr	-70(ra) # 40000354 <memset>
420093a2:	04000613          	li	a2,64
420093a6:	4581                	li	a1,0
420093a8:	0828                	add	a0,sp,24
420093aa:	fdff7097          	auipc	ra,0xfdff7
420093ae:	faa080e7          	jalr	-86(ra) # 40000354 <memset>
# 	int bad_bits = 0, first_bad = -1, wrong_level = 0;
420093b2:	8922                	mv	s2,s0
# 	for (size_t i = 0; i < n; i++) {
420093b4:	4701                	li	a4,0
# 	int bad_bits = 0, first_bad = -1, wrong_level = 0;
420093b6:	59fd                	li	s3,-1
# 	for (size_t i = 0; i < n; i++) {
420093b8:	a0ad                	j	42009422 <cmd_led_probe+0x170>
# 		send_frame_locked(frame, sizeof(frame));
420093ba:	45e1                	li	a1,24
420093bc:	1128                	add	a0,sp,168
420093be:	d0fff0ef          	jal	420090cc <send_frame_locked>
# 		show_locked();   /* put the real state back */
420093c2:	d8dff0ef          	jal	4200914e <show_locked>
420093c6:	bf59                	j	4200935c <cmd_led_probe+0xaa>
# 		printf("probe: rmt_receive: %s\n", esp_err_to_name(err));
420093c8:	8522                	mv	a0,s0
420093ca:	c93f80ef          	jal	4200205c <esp_err_to_name>
420093ce:	85aa                	mv	a1,a0
420093d0:	3c075537          	lui	a0,0x3c075
420093d4:	35050513          	add	a0,a0,848 # 3c075350 <_flash_rodata_start+0x5230>
420093d8:	32b510ef          	jal	4205af02 <printf>
# 		return 1;
420093dc:	4485                	li	s1,1
# }
420093de:	8526                	mv	a0,s1
420093e0:	40fe                	lw	ra,220(sp)
420093e2:	446e                	lw	s0,216(sp)
420093e4:	44de                	lw	s1,212(sp)
420093e6:	494e                	lw	s2,208(sp)
420093e8:	49be                	lw	s3,204(sp)
420093ea:	612d                	add	sp,sp,224
420093ec:	8082                	ret
# 		rmt_disable(probe_chan);
420093ee:	3fc99437          	lui	s0,0x3fc99
420093f2:	38042503          	lw	a0,896(s0) # 3fc99380 <probe_chan>
420093f6:	5151b0ef          	jal	4202510a <rmt_disable>
# 		rmt_enable(probe_chan);
420093fa:	38042503          	lw	a0,896(s0)
420093fe:	4c31b0ef          	jal	420250c0 <rmt_enable>
# 		printf("sent %d bits on GPIO%d, nothing arrived on GPIO%d.\n"
42009402:	4799                	li	a5,6
42009404:	4715                	li	a4,5
42009406:	4699                	li	a3,6
42009408:	4615                	li	a2,5
4200940a:	0c000593          	li	a1,192
4200940e:	3c075537          	lui	a0,0x3c075
42009412:	36850513          	add	a0,a0,872 # 3c075368 <_flash_rodata_start+0x5248>
42009416:	2ed510ef          	jal	4205af02 <printf>
# 		return 1;
4200941a:	4485                	li	s1,1
4200941c:	b7c9                	j	420093de <cmd_led_probe+0x12c>
# 			wrong_level++;
4200941e:	0905                	add	s2,s2,1
# 	for (size_t i = 0; i < n; i++) {
42009420:	0705                	add	a4,a4,1
42009422:	46ea                	lw	a3,152(sp)
42009424:	0ad77a63          	bgeu	a4,a3,420094d8 <cmd_led_probe+0x226>
# 		rmt_symbol_word_t s = probe_buf[i];
42009428:	3fc997b7          	lui	a5,0x3fc99
4200942c:	00271613          	sll	a2,a4,0x2
42009430:	acc78793          	add	a5,a5,-1332 # 3fc98acc <probe_buf>
42009434:	97b2                	add	a5,a5,a2
42009436:	438c                	lw	a1,0(a5)
# 		if (s.level0 != 1) {
42009438:	c62e                	sw	a1,12(sp)
4200943a:	01059793          	sll	a5,a1,0x10
4200943e:	fe07d0e3          	bgez	a5,4200941e <cmd_led_probe+0x16c>
# 		hi[s.duration0 < BINS ? s.duration0 : BINS - 1]++;
42009442:	67a1                	lui	a5,0x8
42009444:	17fd                	add	a5,a5,-1 # 7fff <_rtc_fast_length+0x7dff>
42009446:	8fed                	and	a5,a5,a1
42009448:	07c2                	sll	a5,a5,0x10
4200944a:	83c1                	srl	a5,a5,0x10
4200944c:	863e                	mv	a2,a5
4200944e:	453d                	li	a0,15
42009450:	00f57363          	bgeu	a0,a5,42009456 <cmd_led_probe+0x1a4>
42009454:	463d                	li	a2,15
42009456:	0642                	sll	a2,a2,0x10
42009458:	8241                	srl	a2,a2,0x10
4200945a:	060a                	sll	a2,a2,0x2
4200945c:	0c060613          	add	a2,a2,192
42009460:	960a                	add	a2,a2,sp
42009462:	f9862503          	lw	a0,-104(a2)
42009466:	0505                	add	a0,a0,1
42009468:	f8a62c23          	sw	a0,-104(a2)
# 		if (i + 1 < n) {
4200946c:	00170613          	add	a2,a4,1
42009470:	02d67663          	bgeu	a2,a3,4200949c <cmd_led_probe+0x1ea>
# 			lo[s.duration1 < BINS ? s.duration1 : BINS - 1]++;
42009474:	81c1                	srl	a1,a1,0x10
42009476:	66a1                	lui	a3,0x8
42009478:	16fd                	add	a3,a3,-1 # 7fff <_rtc_fast_length+0x7dff>
4200947a:	8df5                	and	a1,a1,a3
4200947c:	86ae                	mv	a3,a1
4200947e:	463d                	li	a2,15
42009480:	00b67363          	bgeu	a2,a1,42009486 <cmd_led_probe+0x1d4>
42009484:	46bd                	li	a3,15
42009486:	06c2                	sll	a3,a3,0x10
42009488:	82c1                	srl	a3,a3,0x10
4200948a:	068a                	sll	a3,a3,0x2
4200948c:	0c068693          	add	a3,a3,192
42009490:	968a                	add	a3,a3,sp
42009492:	f586a603          	lw	a2,-168(a3)
42009496:	0605                	add	a2,a2,1
42009498:	f4c6ac23          	sw	a2,-168(a3)
# 		int bit = 2 * s.duration0 > T_SHORT + T_LONG;
4200949c:	0786                	sll	a5,a5,0x1
4200949e:	00e7a793          	slti	a5,a5,14
420094a2:	0017c793          	xor	a5,a5,1
# 		if (i < PROBE_BITS) {
420094a6:	0bf00693          	li	a3,191
420094aa:	f6e6ebe3          	bltu	a3,a4,42009420 <cmd_led_probe+0x16e>
# 			int want = (frame[i / 8] >> (7 - i % 8)) & 1;
420094ae:	00375693          	srl	a3,a4,0x3
420094b2:	0c068693          	add	a3,a3,192
420094b6:	968a                	add	a3,a3,sp
420094b8:	fe86c683          	lbu	a3,-24(a3)
420094bc:	fff74613          	not	a2,a4
420094c0:	8a1d                	and	a2,a2,7
420094c2:	40c6d6b3          	sra	a3,a3,a2
420094c6:	8a85                	and	a3,a3,1
# 			if (bit != want) {
420094c8:	f4d78ce3          	beq	a5,a3,42009420 <cmd_led_probe+0x16e>
# 				if (first_bad < 0) {
420094cc:	0009c463          	bltz	s3,420094d4 <cmd_led_probe+0x222>
# 				bad_bits++;
420094d0:	0405                	add	s0,s0,1
420094d2:	b7b9                	j	42009420 <cmd_led_probe+0x16e>
# 					first_bad = i;
420094d4:	89ba                	mv	s3,a4
420094d6:	bfed                	j	420094d0 <cmd_led_probe+0x21e>
# 	printf("sent %d bits on GPIO%d, captured %u on GPIO%d\n",
420094d8:	4719                	li	a4,6
420094da:	4615                	li	a2,5
420094dc:	0c000593          	li	a1,192
420094e0:	3c075537          	lui	a0,0x3c075
420094e4:	3c850513          	add	a0,a0,968 # 3c0753c8 <_flash_rodata_start+0x52a8>
420094e8:	21b510ef          	jal	4205af02 <printf>
# 	print_hist("high", hi, BINS);
420094ec:	4641                	li	a2,16
420094ee:	08ac                	add	a1,sp,88
420094f0:	3c075537          	lui	a0,0x3c075
420094f4:	3f850513          	add	a0,a0,1016 # 3c0753f8 <_flash_rodata_start+0x52d8>
420094f8:	d49ff0ef          	jal	42009240 <print_hist>
# 	print_hist("low ", lo, BINS);
420094fc:	4641                	li	a2,16
420094fe:	082c                	add	a1,sp,24
42009500:	3c075537          	lui	a0,0x3c075
42009504:	40050513          	add	a0,a0,1024 # 3c075400 <_flash_rodata_start+0x52e0>
42009508:	d39ff0ef          	jal	42009240 <print_hist>
# 	printf("  (want high 0.4 us = \"0\", 0.9 us = \"1\", lows the other way round)\n");
4200950c:	3c075537          	lui	a0,0x3c075
42009510:	40850513          	add	a0,a0,1032 # 3c075408 <_flash_rodata_start+0x52e8>
42009514:	383510ef          	jal	4205b096 <puts>
# 	if (wrong_level) {
42009518:	02091463          	bnez	s2,42009540 <cmd_led_probe+0x28e>
# 	if (n == PROBE_BITS && bad_bits == 0 && wrong_level == 0) {
4200951c:	476a                	lw	a4,152(sp)
4200951e:	0c000793          	li	a5,192
42009522:	02f70763          	beq	a4,a5,42009550 <cmd_led_probe+0x29e>
# 	if (bad_bits) {
42009526:	e421                	bnez	s0,4200956e <cmd_led_probe+0x2bc>
# 	if (n != PROBE_BITS) {
42009528:	476a                	lw	a4,152(sp)
4200952a:	0c000793          	li	a5,192
4200952e:	eaf708e3          	beq	a4,a5,420093de <cmd_led_probe+0x12c>
# 		printf("bit count mismatch: loose jumper or noise?\n");
42009532:	3c075537          	lui	a0,0x3c075
42009536:	4e850513          	add	a0,a0,1256 # 3c0754e8 <_flash_rodata_start+0x53c8>
4200953a:	35d510ef          	jal	4205b096 <puts>
4200953e:	b545                	j	420093de <cmd_led_probe+0x12c>
# 		printf("%d pulses started low: inverted signal?\n", wrong_level);
42009540:	85ca                	mv	a1,s2
42009542:	3c075537          	lui	a0,0x3c075
42009546:	44c50513          	add	a0,a0,1100 # 3c07544c <_flash_rodata_start+0x532c>
4200954a:	1b9510ef          	jal	4205af02 <printf>
4200954e:	b7f9                	j	4200951c <cmd_led_probe+0x26a>
# 	if (n == PROBE_BITS && bad_bits == 0 && wrong_level == 0) {
42009550:	01246933          	or	s2,s0,s2
42009554:	fc0919e3          	bnez	s2,42009526 <cmd_led_probe+0x274>
# 		printf("OK: all %d bits match, the data leaves GPIO%d correctly\n",
42009558:	4615                	li	a2,5
4200955a:	0c000593          	li	a1,192
4200955e:	3c075537          	lui	a0,0x3c075
42009562:	47850513          	add	a0,a0,1144 # 3c075478 <_flash_rodata_start+0x5358>
42009566:	19d510ef          	jal	4205af02 <printf>
# 		return 0;
4200956a:	84ca                	mv	s1,s2
4200956c:	bd8d                	j	420093de <cmd_led_probe+0x12c>
# 		       bad_bits, first_bad, first_bad / 24, first_bad / 8 % 3);
4200956e:	41f9d793          	sra	a5,s3,0x1f
42009572:	8b9d                	and	a5,a5,7
42009574:	97ce                	add	a5,a5,s3
42009576:	878d                	sra	a5,a5,0x3
# 		printf("%d bits wrong, first at bit %d (LED %d, byte %d)\n",
42009578:	470d                	li	a4,3
4200957a:	02e7e733          	rem	a4,a5,a4
4200957e:	46e1                	li	a3,24
42009580:	02d9c6b3          	div	a3,s3,a3
42009584:	864e                	mv	a2,s3
42009586:	85a2                	mv	a1,s0
42009588:	3c075537          	lui	a0,0x3c075
4200958c:	4b450513          	add	a0,a0,1204 # 3c0754b4 <_flash_rodata_start+0x5394>
42009590:	173510ef          	jal	4205af02 <printf>
42009594:	bf51                	j	42009528 <cmd_led_probe+0x276>

42009596 <parse_index>:
# {
42009596:	1101                	add	sp,sp,-32
42009598:	ce06                	sw	ra,28(sp)
4200959a:	cc22                	sw	s0,24(sp)
4200959c:	ca26                	sw	s1,20(sp)
4200959e:	84aa                	mv	s1,a0
420095a0:	842e                	mv	s0,a1
# 	if (strcasecmp(s, "all") == 0) {
420095a2:	3c0755b7          	lui	a1,0x3c075
420095a6:	51458593          	add	a1,a1,1300 # 3c075514 <_flash_rodata_start+0x53f4>
420095aa:	fdff7097          	auipc	ra,0xfdff7
420095ae:	e26080e7          	jalr	-474(ra) # 400003d0 <strcasecmp>
420095b2:	e911                	bnez	a0,420095c6 <parse_index+0x30>
# 		*out = LEDS_ALL;
420095b4:	57fd                	li	a5,-1
420095b6:	00f40023          	sb	a5,0(s0)
# 		return true;
420095ba:	4505                	li	a0,1
# }
420095bc:	40f2                	lw	ra,28(sp)
420095be:	4462                	lw	s0,24(sp)
420095c0:	44d2                	lw	s1,20(sp)
420095c2:	6105                	add	sp,sp,32
420095c4:	8082                	ret
# 	long v = strtol(s, &end, 10);
420095c6:	4629                	li	a2,10
420095c8:	006c                	add	a1,sp,12
420095ca:	8526                	mv	a0,s1
420095cc:	fdff7097          	auipc	ra,0xfdff7
420095d0:	e88080e7          	jalr	-376(ra) # 40000454 <strtol>
# 	if (*end != '\0' || v < 0 || v >= LEDS_COUNT) {
420095d4:	47b2                	lw	a5,12(sp)
420095d6:	0007c783          	lbu	a5,0(a5)
420095da:	eb91                	bnez	a5,420095ee <parse_index+0x58>
420095dc:	00054b63          	bltz	a0,420095f2 <parse_index+0x5c>
420095e0:	479d                	li	a5,7
420095e2:	00a7ca63          	blt	a5,a0,420095f6 <parse_index+0x60>
# 	*out = (uint8_t)v;
420095e6:	00a40023          	sb	a0,0(s0)
# 	return true;
420095ea:	4505                	li	a0,1
420095ec:	bfc1                	j	420095bc <parse_index+0x26>
# 		return false;
420095ee:	4501                	li	a0,0
420095f0:	b7f1                	j	420095bc <parse_index+0x26>
420095f2:	4501                	li	a0,0
420095f4:	b7e1                	j	420095bc <parse_index+0x26>
420095f6:	4501                	li	a0,0
420095f8:	b7d1                	j	420095bc <parse_index+0x26>

420095fa <parse_byte>:
# {
420095fa:	1101                	add	sp,sp,-32
420095fc:	ce06                	sw	ra,28(sp)
420095fe:	cc22                	sw	s0,24(sp)
42009600:	842e                	mv	s0,a1
# 	long v = strtol(s, &end, 0);
42009602:	4601                	li	a2,0
42009604:	006c                	add	a1,sp,12
42009606:	fdff7097          	auipc	ra,0xfdff7
4200960a:	e4e080e7          	jalr	-434(ra) # 40000454 <strtol>
# 	if (*end != '\0' || v < 0 || v > 255) {
4200960e:	47b2                	lw	a5,12(sp)
42009610:	0007c783          	lbu	a5,0(a5)
42009614:	eb99                	bnez	a5,4200962a <parse_byte+0x30>
42009616:	00054f63          	bltz	a0,42009634 <parse_byte+0x3a>
4200961a:	0ff00793          	li	a5,255
4200961e:	00a7cd63          	blt	a5,a0,42009638 <parse_byte+0x3e>
# 	*out = (uint8_t)v;
42009622:	00a40023          	sb	a0,0(s0)
# 	return true;
42009626:	4505                	li	a0,1
42009628:	a011                	j	4200962c <parse_byte+0x32>
# 		return false;
4200962a:	4501                	li	a0,0
# }
4200962c:	40f2                	lw	ra,28(sp)
4200962e:	4462                	lw	s0,24(sp)
42009630:	6105                	add	sp,sp,32
42009632:	8082                	ret
# 		return false;
42009634:	4501                	li	a0,0
42009636:	bfdd                	j	4200962c <parse_byte+0x32>
42009638:	4501                	li	a0,0
4200963a:	bfcd                	j	4200962c <parse_byte+0x32>

4200963c <leds_init>:
# {
4200963c:	7139                	add	sp,sp,-64
4200963e:	de06                	sw	ra,60(sp)
42009640:	dc22                	sw	s0,56(sp)
# 	lock = xSemaphoreCreateMutex();
42009642:	4505                	li	a0,1
42009644:	fe384097          	auipc	ra,0xfe384
42009648:	b72080e7          	jalr	-1166(ra) # 4038d1b6 <xQueueCreateMutex>
4200964c:	3fc997b7          	lui	a5,0x3fc99
42009650:	38a7a623          	sw	a0,908(a5) # 3fc9938c <lock>
# 	rmt_tx_channel_config_t ch_cfg = {
42009654:	ca02                	sw	zero,20(sp)
42009656:	cc02                	sw	zero,24(sp)
42009658:	ce02                	sw	zero,28(sp)
4200965a:	d002                	sw	zero,32(sp)
4200965c:	d202                	sw	zero,36(sp)
4200965e:	d402                	sw	zero,40(sp)
42009660:	d602                	sw	zero,44(sp)
42009662:	4795                	li	a5,5
42009664:	ca3e                	sw	a5,20(sp)
42009666:	4711                	li	a4,4
42009668:	cc3a                	sw	a4,24(sp)
4200966a:	009897b7          	lui	a5,0x989
4200966e:	68078793          	add	a5,a5,1664 # 989680 <_rtc_fast_length+0x989480>
42009672:	ce3e                	sw	a5,28(sp)
42009674:	03000793          	li	a5,48
42009678:	d03e                	sw	a5,32(sp)
4200967a:	d23a                	sw	a4,36(sp)
# 	ESP_ERROR_CHECK(rmt_new_tx_channel(&ch_cfg, &chan));
4200967c:	3fc995b7          	lui	a1,0x3fc99
42009680:	39458593          	add	a1,a1,916 # 3fc99394 <chan>
42009684:	0848                	add	a0,sp,20
42009686:	7ce1d0ef          	jal	42026e54 <rmt_new_tx_channel>
4200968a:	ed41                	bnez	a0,42009722 <leds_init+0xe6>
# 	rmt_bytes_encoder_config_t enc_cfg = {
4200968c:	75e1                	lui	a1,0xffff8
4200968e:	47a2                	lw	a5,8(sp)
42009690:	8fed                	and	a5,a5,a1
42009692:	0047e793          	or	a5,a5,4
42009696:	c43e                	sw	a5,8(sp)
42009698:	6621                	lui	a2,0x8
4200969a:	8fd1                	or	a5,a5,a2
4200969c:	c43e                	sw	a5,8(sp)
4200969e:	800106b7          	lui	a3,0x80010
420096a2:	16fd                	add	a3,a3,-1 # 8000ffff <SYSTEM+0x1ff4ffff>
420096a4:	8ff5                	and	a5,a5,a3
420096a6:	00090737          	lui	a4,0x90
420096aa:	8fd9                	or	a5,a5,a4
420096ac:	c43e                	sw	a5,8(sp)
420096ae:	80000737          	lui	a4,0x80000
420096b2:	177d                	add	a4,a4,-1 # 7fffffff <SYSTEM+0x1ff3ffff>
420096b4:	8ff9                	and	a5,a5,a4
420096b6:	c43e                	sw	a5,8(sp)
420096b8:	47b2                	lw	a5,12(sp)
420096ba:	8fed                	and	a5,a5,a1
420096bc:	0097e793          	or	a5,a5,9
420096c0:	c63e                	sw	a5,12(sp)
420096c2:	8fd1                	or	a5,a5,a2
420096c4:	c63e                	sw	a5,12(sp)
420096c6:	8ff5                	and	a5,a5,a3
420096c8:	000406b7          	lui	a3,0x40
420096cc:	8fd5                	or	a5,a5,a3
420096ce:	c63e                	sw	a5,12(sp)
420096d0:	8ff9                	and	a5,a5,a4
420096d2:	c63e                	sw	a5,12(sp)
420096d4:	01014783          	lbu	a5,16(sp)
420096d8:	0017e793          	or	a5,a5,1
420096dc:	00f10823          	sb	a5,16(sp)
# 	ESP_ERROR_CHECK(rmt_new_bytes_encoder(&enc_cfg, &encoder));
420096e0:	3fc995b7          	lui	a1,0x3fc99
420096e4:	39058593          	add	a1,a1,912 # 3fc99390 <encoder>
420096e8:	0028                	add	a0,sp,8
420096ea:	3491b0ef          	jal	42025232 <rmt_new_bytes_encoder>
420096ee:	ed21                	bnez	a0,42009746 <leds_init+0x10a>
# 	ESP_ERROR_CHECK(rmt_enable(chan));
420096f0:	3fc997b7          	lui	a5,0x3fc99
420096f4:	3947a503          	lw	a0,916(a5) # 3fc99394 <chan>
420096f8:	1c91b0ef          	jal	420250c0 <rmt_enable>
420096fc:	c541                	beqz	a0,42009784 <leds_init+0x148>
420096fe:	3c075737          	lui	a4,0x3c075
42009702:	57070713          	add	a4,a4,1392 # 3c075570 <_flash_rodata_start+0x5450>
42009706:	3c07e6b7          	lui	a3,0x3c07e
4200970a:	c8868693          	add	a3,a3,-888 # 3c07dc88 <__func__.5>
4200970e:	07c00613          	li	a2,124
42009712:	3c0755b7          	lui	a1,0x3c075
42009716:	2f058593          	add	a1,a1,752 # 3c0752f0 <_flash_rodata_start+0x51d0>
4200971a:	fe383097          	auipc	ra,0xfe383
4200971e:	2d4080e7          	jalr	724(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(rmt_new_tx_channel(&ch_cfg, &chan));
42009722:	3c075737          	lui	a4,0x3c075
42009726:	52070713          	add	a4,a4,1312 # 3c075520 <_flash_rodata_start+0x5400>
4200972a:	3c07e6b7          	lui	a3,0x3c07e
4200972e:	c8868693          	add	a3,a3,-888 # 3c07dc88 <__func__.5>
42009732:	07200613          	li	a2,114
42009736:	3c0755b7          	lui	a1,0x3c075
4200973a:	2f058593          	add	a1,a1,752 # 3c0752f0 <_flash_rodata_start+0x51d0>
4200973e:	fe383097          	auipc	ra,0xfe383
42009742:	2b0080e7          	jalr	688(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(rmt_new_bytes_encoder(&enc_cfg, &encoder));
42009746:	3c075737          	lui	a4,0x3c075
4200974a:	54470713          	add	a4,a4,1348 # 3c075544 <_flash_rodata_start+0x5424>
4200974e:	3c07e6b7          	lui	a3,0x3c07e
42009752:	c8868693          	add	a3,a3,-888 # 3c07dc88 <__func__.5>
42009756:	07b00613          	li	a2,123
4200975a:	3c0755b7          	lui	a1,0x3c075
4200975e:	2f058593          	add	a1,a1,752 # 3c0752f0 <_flash_rodata_start+0x51d0>
42009762:	fe383097          	auipc	ra,0xfe383
42009766:	28c080e7          	jalr	652(ra) # 4038c9ee <_esp_error_check_failed>
# 		leds[i] = (led_state_t){ .on = false, .r = 255, .g = 255, .b = 255 };
4200976a:	3fc997b7          	lui	a5,0x3fc99
4200976e:	00251713          	sll	a4,a0,0x2
42009772:	e0c78793          	add	a5,a5,-500 # 3fc98e0c <leds>
42009776:	97ba                	add	a5,a5,a4
42009778:	3c07e737          	lui	a4,0x3c07e
4200977c:	cd472703          	lw	a4,-812(a4) # 3c07dcd4 <colors+0x40>
42009780:	c398                	sw	a4,0(a5)
# 	for (int i = 0; i < LEDS_COUNT; i++) {
42009782:	0505                	add	a0,a0,1
42009784:	479d                	li	a5,7
42009786:	fea7d2e3          	bge	a5,a0,4200976a <leds_init+0x12e>
# 	xSemaphoreTake(lock, portMAX_DELAY);
4200978a:	3fc99437          	lui	s0,0x3fc99
4200978e:	55fd                	li	a1,-1
42009790:	38c42503          	lw	a0,908(s0) # 3fc9938c <lock>
42009794:	fe384097          	auipc	ra,0xfe384
42009798:	ee8080e7          	jalr	-280(ra) # 4038d67c <xQueueSemaphoreTake>
# 	show_locked();
4200979c:	9b3ff0ef          	jal	4200914e <show_locked>
# 	xSemaphoreGive(lock);
420097a0:	4681                	li	a3,0
420097a2:	4601                	li	a2,0
420097a4:	4581                	li	a1,0
420097a6:	38c42503          	lw	a0,908(s0)
420097aa:	fe384097          	auipc	ra,0xfe384
420097ae:	806080e7          	jalr	-2042(ra) # 4038cfb0 <xQueueGenericSend>
# 	ESP_LOGI(TAG, "%d LEDs on GPIO%d, %s order", LEDS_COUNT, LEDS_GPIO,
420097b2:	fe38a097          	auipc	ra,0xfe38a
420097b6:	f2a080e7          	jalr	-214(ra) # 403936dc <esp_log_timestamp>
420097ba:	86aa                	mv	a3,a0
420097bc:	3fc997b7          	lui	a5,0x3fc99
420097c0:	3887c783          	lbu	a5,904(a5) # 3fc99388 <order_rgb>
420097c4:	cb95                	beqz	a5,420097f8 <leds_init+0x1bc>
420097c6:	3c0758b7          	lui	a7,0x3c075
420097ca:	51888893          	add	a7,a7,1304 # 3c075518 <_flash_rodata_start+0x53f8>
420097ce:	4815                	li	a6,5
420097d0:	47a1                	li	a5,8
420097d2:	3c0755b7          	lui	a1,0x3c075
420097d6:	0f458713          	add	a4,a1,244 # 3c0750f4 <_flash_rodata_start+0x4fd4>
420097da:	3c075637          	lui	a2,0x3c075
420097de:	58460613          	add	a2,a2,1412 # 3c075584 <_flash_rodata_start+0x5464>
420097e2:	0f458593          	add	a1,a1,244
420097e6:	450d                	li	a0,3
420097e8:	fe38a097          	auipc	ra,0xfe38a
420097ec:	e06080e7          	jalr	-506(ra) # 403935ee <esp_log_write>
# }
420097f0:	50f2                	lw	ra,60(sp)
420097f2:	5462                	lw	s0,56(sp)
420097f4:	6121                	add	sp,sp,64
420097f6:	8082                	ret
# 	ESP_LOGI(TAG, "%d LEDs on GPIO%d, %s order", LEDS_COUNT, LEDS_GPIO,
420097f8:	3c0758b7          	lui	a7,0x3c075
420097fc:	51c88893          	add	a7,a7,1308 # 3c07551c <_flash_rodata_start+0x53fc>
42009800:	b7f9                	j	420097ce <leds_init+0x192>

42009802 <leds_set_on>:
# {
42009802:	1141                	add	sp,sp,-16
42009804:	c606                	sw	ra,12(sp)
42009806:	c422                	sw	s0,8(sp)
42009808:	c226                	sw	s1,4(sp)
4200980a:	842a                	mv	s0,a0
4200980c:	84ae                	mv	s1,a1
# 	if (index != LEDS_ALL && index >= LEDS_COUNT) {
4200980e:	ff850793          	add	a5,a0,-8
42009812:	0ff7f793          	zext.b	a5,a5
42009816:	0f600713          	li	a4,246
4200981a:	00f76863          	bltu	a4,a5,4200982a <leds_set_on+0x28>
# 		return false;
4200981e:	4501                	li	a0,0
# }
42009820:	40b2                	lw	ra,12(sp)
42009822:	4422                	lw	s0,8(sp)
42009824:	4492                	lw	s1,4(sp)
42009826:	0141                	add	sp,sp,16
42009828:	8082                	ret
# 	xSemaphoreTake(lock, portMAX_DELAY);
4200982a:	55fd                	li	a1,-1
4200982c:	3fc997b7          	lui	a5,0x3fc99
42009830:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
42009834:	fe384097          	auipc	ra,0xfe384
42009838:	e48080e7          	jalr	-440(ra) # 4038d67c <xQueueSemaphoreTake>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
4200983c:	4781                	li	a5,0
4200983e:	a819                	j	42009854 <leds_set_on+0x52>
# 			leds[i].on = on;
42009840:	3fc99737          	lui	a4,0x3fc99
42009844:	00279693          	sll	a3,a5,0x2
42009848:	e0c70713          	add	a4,a4,-500 # 3fc98e0c <leds>
4200984c:	9736                	add	a4,a4,a3
4200984e:	00970023          	sb	s1,0(a4)
# 	for (int i = 0; i < LEDS_COUNT; i++) {
42009852:	0785                	add	a5,a5,1
42009854:	471d                	li	a4,7
42009856:	00f74963          	blt	a4,a5,42009868 <leds_set_on+0x66>
# 		if (index == LEDS_ALL || index == i) {
4200985a:	0ff00713          	li	a4,255
4200985e:	fee401e3          	beq	s0,a4,42009840 <leds_set_on+0x3e>
42009862:	fef418e3          	bne	s0,a5,42009852 <leds_set_on+0x50>
42009866:	bfe9                	j	42009840 <leds_set_on+0x3e>
# 	show_locked();
42009868:	8e7ff0ef          	jal	4200914e <show_locked>
# 	xSemaphoreGive(lock);
4200986c:	4681                	li	a3,0
4200986e:	4601                	li	a2,0
42009870:	4581                	li	a1,0
42009872:	3fc997b7          	lui	a5,0x3fc99
42009876:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
4200987a:	fe383097          	auipc	ra,0xfe383
4200987e:	736080e7          	jalr	1846(ra) # 4038cfb0 <xQueueGenericSend>
# 	changed();
42009882:	835ff0ef          	jal	420090b6 <changed>
# 	return true;
42009886:	4505                	li	a0,1
42009888:	bf61                	j	42009820 <leds_set_on+0x1e>

4200988a <leds_set_color>:
# {
4200988a:	1101                	add	sp,sp,-32
4200988c:	ce06                	sw	ra,28(sp)
4200988e:	cc22                	sw	s0,24(sp)
42009890:	ca26                	sw	s1,20(sp)
42009892:	c84a                	sw	s2,16(sp)
42009894:	c64e                	sw	s3,12(sp)
42009896:	842a                	mv	s0,a0
42009898:	84ae                	mv	s1,a1
4200989a:	8932                	mv	s2,a2
4200989c:	89b6                	mv	s3,a3
# 	if (index != LEDS_ALL && index >= LEDS_COUNT) {
4200989e:	ff850793          	add	a5,a0,-8
420098a2:	0ff7f793          	zext.b	a5,a5
420098a6:	0f600713          	li	a4,246
420098aa:	00f76a63          	bltu	a4,a5,420098be <leds_set_color+0x34>
# 		return false;
420098ae:	4501                	li	a0,0
# }
420098b0:	40f2                	lw	ra,28(sp)
420098b2:	4462                	lw	s0,24(sp)
420098b4:	44d2                	lw	s1,20(sp)
420098b6:	4942                	lw	s2,16(sp)
420098b8:	49b2                	lw	s3,12(sp)
420098ba:	6105                	add	sp,sp,32
420098bc:	8082                	ret
# 	xSemaphoreTake(lock, portMAX_DELAY);
420098be:	55fd                	li	a1,-1
420098c0:	3fc997b7          	lui	a5,0x3fc99
420098c4:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
420098c8:	fe384097          	auipc	ra,0xfe384
420098cc:	db4080e7          	jalr	-588(ra) # 4038d67c <xQueueSemaphoreTake>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
420098d0:	4701                	li	a4,0
420098d2:	a015                	j	420098f6 <leds_set_color+0x6c>
# 			leds[i] = (led_state_t){ .on = true, .r = r, .g = g, .b = b };
420098d4:	3fc997b7          	lui	a5,0x3fc99
420098d8:	00271513          	sll	a0,a4,0x2
420098dc:	e0c78793          	add	a5,a5,-500 # 3fc98e0c <leds>
420098e0:	97aa                	add	a5,a5,a0
420098e2:	4685                	li	a3,1
420098e4:	00d78023          	sb	a3,0(a5)
420098e8:	009780a3          	sb	s1,1(a5)
420098ec:	01278123          	sb	s2,2(a5)
420098f0:	013781a3          	sb	s3,3(a5)
# 	for (int i = 0; i < LEDS_COUNT; i++) {
420098f4:	0705                	add	a4,a4,1
420098f6:	479d                	li	a5,7
420098f8:	00e7c963          	blt	a5,a4,4200990a <leds_set_color+0x80>
# 		if (index == LEDS_ALL || index == i) {
420098fc:	0ff00793          	li	a5,255
42009900:	fcf40ae3          	beq	s0,a5,420098d4 <leds_set_color+0x4a>
42009904:	fee418e3          	bne	s0,a4,420098f4 <leds_set_color+0x6a>
42009908:	b7f1                	j	420098d4 <leds_set_color+0x4a>
# 	show_locked();
4200990a:	845ff0ef          	jal	4200914e <show_locked>
# 	xSemaphoreGive(lock);
4200990e:	4681                	li	a3,0
42009910:	4601                	li	a2,0
42009912:	4581                	li	a1,0
42009914:	3fc997b7          	lui	a5,0x3fc99
42009918:	38c7a503          	lw	a0,908(a5) # 3fc9938c <lock>
4200991c:	fe383097          	auipc	ra,0xfe383
42009920:	694080e7          	jalr	1684(ra) # 4038cfb0 <xQueueGenericSend>
# 	changed();
42009924:	f92ff0ef          	jal	420090b6 <changed>
# 	return true;
42009928:	4505                	li	a0,1
4200992a:	b759                	j	420098b0 <leds_set_color+0x26>

4200992c <cmd_led_test>:
# {
4200992c:	1141                	add	sp,sp,-16
4200992e:	c606                	sw	ra,12(sp)
42009930:	c422                	sw	s0,8(sp)
42009932:	c226                	sw	s1,4(sp)
# 	leds_set_on(LEDS_ALL, false);
42009934:	4581                	li	a1,0
42009936:	0ff00513          	li	a0,255
4200993a:	ec9ff0ef          	jal	42009802 <leds_set_on>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
4200993e:	4481                	li	s1,0
42009940:	a095                	j	420099a4 <cmd_led_test+0x78>
# 			printf("LED %d %s\n", i, names[c]);
42009942:	3c07e7b7          	lui	a5,0x3c07e
42009946:	00241713          	sll	a4,s0,0x2
4200994a:	c6078793          	add	a5,a5,-928 # 3c07dc60 <names.3>
4200994e:	97ba                	add	a5,a5,a4
42009950:	4390                	lw	a2,0(a5)
42009952:	85a6                	mv	a1,s1
42009954:	3c075537          	lui	a0,0x3c075
42009958:	5b850513          	add	a0,a0,1464 # 3c0755b8 <_flash_rodata_start+0x5498>
4200995c:	5a6510ef          	jal	4205af02 <printf>
# 			leds_set_color(i, seq[c][0], seq[c][1], seq[c][2]);
42009960:	3c07e7b7          	lui	a5,0x3c07e
42009964:	00141713          	sll	a4,s0,0x1
42009968:	9722                	add	a4,a4,s0
4200996a:	c5478793          	add	a5,a5,-940 # 3c07dc54 <seq.2>
4200996e:	97ba                	add	a5,a5,a4
42009970:	0027c683          	lbu	a3,2(a5)
42009974:	0017c603          	lbu	a2,1(a5)
42009978:	0007c583          	lbu	a1,0(a5)
4200997c:	0ff4f513          	zext.b	a0,s1
42009980:	f0bff0ef          	jal	4200988a <leds_set_color>
# 			vTaskDelay(pdMS_TO_TICKS(400));
42009984:	02800513          	li	a0,40
42009988:	fe385097          	auipc	ra,0xfe385
4200998c:	256080e7          	jalr	598(ra) # 4038ebde <vTaskDelay>
# 		for (int c = 0; c < 3; c++) {
42009990:	0405                	add	s0,s0,1
42009992:	4789                	li	a5,2
42009994:	fa87d7e3          	bge	a5,s0,42009942 <cmd_led_test+0x16>
# 		leds_set_on(i, false);
42009998:	4581                	li	a1,0
4200999a:	0ff4f513          	zext.b	a0,s1
4200999e:	e65ff0ef          	jal	42009802 <leds_set_on>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
420099a2:	0485                	add	s1,s1,1
420099a4:	479d                	li	a5,7
420099a6:	0097c463          	blt	a5,s1,420099ae <cmd_led_test+0x82>
# 		for (int c = 0; c < 3; c++) {
420099aa:	4401                	li	s0,0
420099ac:	b7dd                	j	42009992 <cmd_led_test+0x66>
# }
420099ae:	4501                	li	a0,0
420099b0:	40b2                	lw	ra,12(sp)
420099b2:	4422                	lw	s0,8(sp)
420099b4:	4492                	lw	s1,4(sp)
420099b6:	0141                	add	sp,sp,16
420099b8:	8082                	ret

420099ba <leds_set_brightness>:
# {
420099ba:	1141                	add	sp,sp,-16
420099bc:	c606                	sw	ra,12(sp)
420099be:	c422                	sw	s0,8(sp)
420099c0:	c226                	sw	s1,4(sp)
420099c2:	842a                	mv	s0,a0
# 	xSemaphoreTake(lock, portMAX_DELAY);
420099c4:	3fc994b7          	lui	s1,0x3fc99
420099c8:	55fd                	li	a1,-1
420099ca:	38c4a503          	lw	a0,908(s1) # 3fc9938c <lock>
420099ce:	fe384097          	auipc	ra,0xfe384
420099d2:	cae080e7          	jalr	-850(ra) # 4038d67c <xQueueSemaphoreTake>
# 	brightness = value;
420099d6:	84818823          	sb	s0,-1968(gp) # 3fc95250 <brightness>
# 	show_locked();
420099da:	f74ff0ef          	jal	4200914e <show_locked>
# 	xSemaphoreGive(lock);
420099de:	4681                	li	a3,0
420099e0:	4601                	li	a2,0
420099e2:	4581                	li	a1,0
420099e4:	38c4a503          	lw	a0,908(s1)
420099e8:	fe383097          	auipc	ra,0xfe383
420099ec:	5c8080e7          	jalr	1480(ra) # 4038cfb0 <xQueueGenericSend>
# 	changed();
420099f0:	ec6ff0ef          	jal	420090b6 <changed>
# }
420099f4:	40b2                	lw	ra,12(sp)
420099f6:	4422                	lw	s0,8(sp)
420099f8:	4492                	lw	s1,4(sp)
420099fa:	0141                	add	sp,sp,16
420099fc:	8082                	ret

420099fe <leds_count>:
# }
420099fe:	4521                	li	a0,8
42009a00:	8082                	ret

42009a02 <leds_brightness>:
# }
42009a02:	8501c503          	lbu	a0,-1968(gp) # 3fc95250 <brightness>
42009a06:	8082                	ret

42009a08 <leds_get>:
# {
42009a08:	1101                	add	sp,sp,-32
42009a0a:	ce06                	sw	ra,28(sp)
42009a0c:	cc22                	sw	s0,24(sp)
42009a0e:	ca26                	sw	s1,20(sp)
42009a10:	842a                	mv	s0,a0
# 	led_state_t s = { 0 };
42009a12:	c402                	sw	zero,8(sp)
# 	if (index < LEDS_COUNT) {
42009a14:	479d                	li	a5,7
42009a16:	02a7f763          	bgeu	a5,a0,42009a44 <leds_get+0x3c>
# 	return s;
42009a1a:	47a2                	lw	a5,8(sp)
42009a1c:	c63e                	sw	a5,12(sp)
42009a1e:	00c14783          	lbu	a5,12(sp)
42009a22:	00d14683          	lbu	a3,13(sp)
42009a26:	06a2                	sll	a3,a3,0x8
42009a28:	00e14703          	lbu	a4,14(sp)
42009a2c:	0742                	sll	a4,a4,0x10
42009a2e:	8fd5                	or	a5,a5,a3
42009a30:	00f14503          	lbu	a0,15(sp)
42009a34:	0562                	sll	a0,a0,0x18
42009a36:	8fd9                	or	a5,a5,a4
# }
42009a38:	8d5d                	or	a0,a0,a5
42009a3a:	40f2                	lw	ra,28(sp)
42009a3c:	4462                	lw	s0,24(sp)
42009a3e:	44d2                	lw	s1,20(sp)
42009a40:	6105                	add	sp,sp,32
42009a42:	8082                	ret
# 		xSemaphoreTake(lock, portMAX_DELAY);
42009a44:	3fc994b7          	lui	s1,0x3fc99
42009a48:	55fd                	li	a1,-1
42009a4a:	38c4a503          	lw	a0,908(s1) # 3fc9938c <lock>
42009a4e:	fe384097          	auipc	ra,0xfe384
42009a52:	c2e080e7          	jalr	-978(ra) # 4038d67c <xQueueSemaphoreTake>
# 		s = leds[index];
42009a56:	3fc997b7          	lui	a5,0x3fc99
42009a5a:	040a                	sll	s0,s0,0x2
42009a5c:	e0c78793          	add	a5,a5,-500 # 3fc98e0c <leds>
42009a60:	97a2                	add	a5,a5,s0
42009a62:	439c                	lw	a5,0(a5)
42009a64:	c43e                	sw	a5,8(sp)
# 		xSemaphoreGive(lock);
42009a66:	4681                	li	a3,0
42009a68:	4601                	li	a2,0
42009a6a:	4581                	li	a1,0
42009a6c:	38c4a503          	lw	a0,908(s1)
42009a70:	fe383097          	auipc	ra,0xfe383
42009a74:	540080e7          	jalr	1344(ra) # 4038cfb0 <xQueueGenericSend>
42009a78:	b74d                	j	42009a1a <leds_get+0x12>

42009a7a <print_state>:
# {
42009a7a:	1101                	add	sp,sp,-32
42009a7c:	ce06                	sw	ra,28(sp)
42009a7e:	cc22                	sw	s0,24(sp)
# 	       LEDS_COUNT, LEDS_GPIO, order_rgb ? "RGB" : "GRB", brightness);
42009a80:	3fc997b7          	lui	a5,0x3fc99
42009a84:	3887c783          	lbu	a5,904(a5) # 3fc99388 <order_rgb>
# 	printf("%d LEDs on GPIO%d, %s order, brightness %u/255\n",
42009a88:	c38d                	beqz	a5,42009aaa <print_state+0x30>
42009a8a:	3c0756b7          	lui	a3,0x3c075
42009a8e:	51868693          	add	a3,a3,1304 # 3c075518 <_flash_rodata_start+0x53f8>
42009a92:	8501c703          	lbu	a4,-1968(gp) # 3fc95250 <brightness>
42009a96:	4615                	li	a2,5
42009a98:	45a1                	li	a1,8
42009a9a:	3c075537          	lui	a0,0x3c075
42009a9e:	5c450513          	add	a0,a0,1476 # 3c0755c4 <_flash_rodata_start+0x54a4>
42009aa2:	460510ef          	jal	4205af02 <printf>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
42009aa6:	4401                	li	s0,0
42009aa8:	a805                	j	42009ad8 <print_state+0x5e>
# 	printf("%d LEDs on GPIO%d, %s order, brightness %u/255\n",
42009aaa:	3c0756b7          	lui	a3,0x3c075
42009aae:	51c68693          	add	a3,a3,1308 # 3c07551c <_flash_rodata_start+0x53fc>
42009ab2:	b7c5                	j	42009a92 <print_state+0x18>
# 		printf("  %2d  %-3s  #%02X%02X%02X\n", i, s.on ? "on" : "off", s.r, s.g, s.b);
42009ab4:	3c074637          	lui	a2,0x3c074
42009ab8:	7fc60613          	add	a2,a2,2044 # 3c0747fc <_flash_rodata_start+0x46dc>
42009abc:	00f14783          	lbu	a5,15(sp)
42009ac0:	00e14703          	lbu	a4,14(sp)
42009ac4:	00d14683          	lbu	a3,13(sp)
42009ac8:	85a2                	mv	a1,s0
42009aca:	3c075537          	lui	a0,0x3c075
42009ace:	5f450513          	add	a0,a0,1524 # 3c0755f4 <_flash_rodata_start+0x54d4>
42009ad2:	430510ef          	jal	4205af02 <printf>
# 	for (int i = 0; i < LEDS_COUNT; i++) {
42009ad6:	0405                	add	s0,s0,1
42009ad8:	479d                	li	a5,7
42009ada:	0087cf63          	blt	a5,s0,42009af8 <print_state+0x7e>
# 		led_state_t s = leds_get(i);
42009ade:	0ff47513          	zext.b	a0,s0
42009ae2:	f27ff0ef          	jal	42009a08 <leds_get>
42009ae6:	c62a                	sw	a0,12(sp)
# 		printf("  %2d  %-3s  #%02X%02X%02X\n", i, s.on ? "on" : "off", s.r, s.g, s.b);
42009ae8:	00c14783          	lbu	a5,12(sp)
42009aec:	d7e1                	beqz	a5,42009ab4 <print_state+0x3a>
42009aee:	3c071637          	lui	a2,0x3c071
42009af2:	d4c60613          	add	a2,a2,-692 # 3c070d4c <_flash_rodata_start+0xc2c>
42009af6:	b7d9                	j	42009abc <print_state+0x42>
# }
42009af8:	40f2                	lw	ra,28(sp)
42009afa:	4462                	lw	s0,24(sp)
42009afc:	6105                	add	sp,sp,32
42009afe:	8082                	ret

42009b00 <cmd_led>:
# 
# static int cmd_led(int argc, char **argv)
# {
42009b00:	7179                	add	sp,sp,-48
42009b02:	d606                	sw	ra,44(sp)
42009b04:	d422                	sw	s0,40(sp)
42009b06:	d226                	sw	s1,36(sp)
42009b08:	d04a                	sw	s2,32(sp)
42009b0a:	ce4e                	sw	s3,28(sp)
42009b0c:	84aa                	mv	s1,a0
42009b0e:	892e                	mv	s2,a1
# 	if (argc == 1) {
42009b10:	4785                	li	a5,1
42009b12:	06f50863          	beq	a0,a5,42009b82 <cmd_led+0x82>
# 		print_state();
# 		return 0;
# 	}
# 
# 	if (strcmp(argv[1], "test") == 0) {
42009b16:	0045a983          	lw	s3,4(a1)
42009b1a:	3c0755b7          	lui	a1,0x3c075
42009b1e:	61058593          	add	a1,a1,1552 # 3c075610 <_flash_rodata_start+0x54f0>
42009b22:	854e                	mv	a0,s3
42009b24:	fdff7097          	auipc	ra,0xfdff7
42009b28:	848080e7          	jalr	-1976(ra) # 4000036c <strcmp>
42009b2c:	cd39                	beqz	a0,42009b8a <cmd_led+0x8a>
# 		return cmd_led_test();
# 	}
# 	if (strcmp(argv[1], "probe") == 0) {
42009b2e:	3c0755b7          	lui	a1,0x3c075
42009b32:	61858593          	add	a1,a1,1560 # 3c075618 <_flash_rodata_start+0x54f8>
42009b36:	854e                	mv	a0,s3
42009b38:	fdff7097          	auipc	ra,0xfdff7
42009b3c:	834080e7          	jalr	-1996(ra) # 4000036c <strcmp>
42009b40:	c929                	beqz	a0,42009b92 <cmd_led+0x92>
# 		return cmd_led_probe();
# 	}
# 	if (strcmp(argv[1], "bright") == 0) {
42009b42:	3c0755b7          	lui	a1,0x3c075
42009b46:	62058593          	add	a1,a1,1568 # 3c075620 <_flash_rodata_start+0x5500>
42009b4a:	854e                	mv	a0,s3
42009b4c:	fdff7097          	auipc	ra,0xfdff7
42009b50:	820080e7          	jalr	-2016(ra) # 4000036c <strcmp>
42009b54:	842a                	mv	s0,a0
42009b56:	e539                	bnez	a0,42009ba4 <cmd_led+0xa4>
# 		uint8_t v;
# 		if (argc != 3 || !parse_byte(argv[2], &v)) {
42009b58:	478d                	li	a5,3
42009b5a:	00f49963          	bne	s1,a5,42009b6c <cmd_led+0x6c>
42009b5e:	00b10593          	add	a1,sp,11
42009b62:	00892503          	lw	a0,8(s2)
42009b66:	a95ff0ef          	jal	420095fa <parse_byte>
42009b6a:	e905                	bnez	a0,42009b9a <cmd_led+0x9a>
# 			return usage();
42009b6c:	dfaff0ef          	jal	42009166 <usage>
42009b70:	842a                	mv	s0,a0
# 	if (argc == 5 && parse_byte(argv[2], &r) && parse_byte(argv[3], &g) && parse_byte(argv[4], &b)) {
# 		leds_set_color(idx, r, g, b);
# 		return 0;
# 	}
# 	return usage();
# }
42009b72:	8522                	mv	a0,s0
42009b74:	50b2                	lw	ra,44(sp)
42009b76:	5422                	lw	s0,40(sp)
42009b78:	5492                	lw	s1,36(sp)
42009b7a:	5902                	lw	s2,32(sp)
42009b7c:	49f2                	lw	s3,28(sp)
42009b7e:	6145                	add	sp,sp,48
42009b80:	8082                	ret
# 		print_state();
42009b82:	ef9ff0ef          	jal	42009a7a <print_state>
# 		return 0;
42009b86:	4401                	li	s0,0
42009b88:	b7ed                	j	42009b72 <cmd_led+0x72>
# 		return cmd_led_test();
42009b8a:	da3ff0ef          	jal	4200992c <cmd_led_test>
42009b8e:	842a                	mv	s0,a0
42009b90:	b7cd                	j	42009b72 <cmd_led+0x72>
# 		return cmd_led_probe();
42009b92:	f20ff0ef          	jal	420092b2 <cmd_led_probe>
42009b96:	842a                	mv	s0,a0
42009b98:	bfe9                	j	42009b72 <cmd_led+0x72>
# 		leds_set_brightness(v);
42009b9a:	00b14503          	lbu	a0,11(sp)
42009b9e:	e1dff0ef          	jal	420099ba <leds_set_brightness>
# 		return 0;
42009ba2:	bfc1                	j	42009b72 <cmd_led+0x72>
# 	if (strcmp(argv[1], "order") == 0) {
42009ba4:	3c0755b7          	lui	a1,0x3c075
42009ba8:	62858593          	add	a1,a1,1576 # 3c075628 <_flash_rodata_start+0x5508>
42009bac:	854e                	mv	a0,s3
42009bae:	fdff6097          	auipc	ra,0xfdff6
42009bb2:	7be080e7          	jalr	1982(ra) # 4000036c <strcmp>
42009bb6:	842a                	mv	s0,a0
42009bb8:	e159                	bnez	a0,42009c3e <cmd_led+0x13e>
# 		if (argc != 3 || (strcasecmp(argv[2], "rgb") != 0 && strcasecmp(argv[2], "grb") != 0)) {
42009bba:	478d                	li	a5,3
42009bbc:	02f49863          	bne	s1,a5,42009bec <cmd_led+0xec>
42009bc0:	00892483          	lw	s1,8(s2)
42009bc4:	3c0755b7          	lui	a1,0x3c075
42009bc8:	63058593          	add	a1,a1,1584 # 3c075630 <_flash_rodata_start+0x5510>
42009bcc:	8526                	mv	a0,s1
42009bce:	fdff7097          	auipc	ra,0xfdff7
42009bd2:	802080e7          	jalr	-2046(ra) # 400003d0 <strcasecmp>
42009bd6:	cd19                	beqz	a0,42009bf4 <cmd_led+0xf4>
42009bd8:	3c0755b7          	lui	a1,0x3c075
42009bdc:	63458593          	add	a1,a1,1588 # 3c075634 <_flash_rodata_start+0x5514>
42009be0:	8526                	mv	a0,s1
42009be2:	fdff6097          	auipc	ra,0xfdff6
42009be6:	7ee080e7          	jalr	2030(ra) # 400003d0 <strcasecmp>
42009bea:	c509                	beqz	a0,42009bf4 <cmd_led+0xf4>
# 			return usage();
42009bec:	d7aff0ef          	jal	42009166 <usage>
42009bf0:	842a                	mv	s0,a0
42009bf2:	b741                	j	42009b72 <cmd_led+0x72>
# 		xSemaphoreTake(lock, portMAX_DELAY);
42009bf4:	3fc994b7          	lui	s1,0x3fc99
42009bf8:	55fd                	li	a1,-1
42009bfa:	38c4a503          	lw	a0,908(s1) # 3fc9938c <lock>
42009bfe:	fe384097          	auipc	ra,0xfe384
42009c02:	a7e080e7          	jalr	-1410(ra) # 4038d67c <xQueueSemaphoreTake>
# 		order_rgb = strcasecmp(argv[2], "rgb") == 0;
42009c06:	3c0755b7          	lui	a1,0x3c075
42009c0a:	63058593          	add	a1,a1,1584 # 3c075630 <_flash_rodata_start+0x5510>
42009c0e:	00892503          	lw	a0,8(s2)
42009c12:	fdff6097          	auipc	ra,0xfdff6
42009c16:	7be080e7          	jalr	1982(ra) # 400003d0 <strcasecmp>
42009c1a:	00153513          	seqz	a0,a0
42009c1e:	3fc997b7          	lui	a5,0x3fc99
42009c22:	38a78423          	sb	a0,904(a5) # 3fc99388 <order_rgb>
# 		show_locked();
42009c26:	d28ff0ef          	jal	4200914e <show_locked>
# 		xSemaphoreGive(lock);
42009c2a:	4681                	li	a3,0
42009c2c:	4601                	li	a2,0
42009c2e:	4581                	li	a1,0
42009c30:	38c4a503          	lw	a0,908(s1)
42009c34:	fe383097          	auipc	ra,0xfe383
42009c38:	37c080e7          	jalr	892(ra) # 4038cfb0 <xQueueGenericSend>
# 		return 0;
42009c3c:	bf1d                	j	42009b72 <cmd_led+0x72>
# 	if (argc < 3 || !parse_index(argv[1], &idx)) {
42009c3e:	4789                	li	a5,2
42009c40:	0297d263          	bge	a5,s1,42009c64 <cmd_led+0x164>
42009c44:	00f10593          	add	a1,sp,15
42009c48:	854e                	mv	a0,s3
42009c4a:	94dff0ef          	jal	42009596 <parse_index>
42009c4e:	c919                	beqz	a0,42009c64 <cmd_led+0x164>
# 	if (argc == 3) {
42009c50:	478d                	li	a5,3
42009c52:	00f48d63          	beq	s1,a5,42009c6c <cmd_led+0x16c>
# 	if (argc == 5 && parse_byte(argv[2], &r) && parse_byte(argv[3], &g) && parse_byte(argv[4], &b)) {
42009c56:	4795                	li	a5,5
42009c58:	0af48863          	beq	s1,a5,42009d08 <cmd_led+0x208>
# 	return usage();
42009c5c:	d0aff0ef          	jal	42009166 <usage>
42009c60:	842a                	mv	s0,a0
42009c62:	bf01                	j	42009b72 <cmd_led+0x72>
# 		return usage();
42009c64:	d02ff0ef          	jal	42009166 <usage>
42009c68:	842a                	mv	s0,a0
42009c6a:	b721                	j	42009b72 <cmd_led+0x72>
# 		if (strcasecmp(argv[2], "on") == 0) {
42009c6c:	00892903          	lw	s2,8(s2)
42009c70:	3c0715b7          	lui	a1,0x3c071
42009c74:	d4c58593          	add	a1,a1,-692 # 3c070d4c <_flash_rodata_start+0xc2c>
42009c78:	854a                	mv	a0,s2
42009c7a:	fdff6097          	auipc	ra,0xfdff6
42009c7e:	756080e7          	jalr	1878(ra) # 400003d0 <strcasecmp>
42009c82:	842a                	mv	s0,a0
42009c84:	c129                	beqz	a0,42009cc6 <cmd_led+0x1c6>
# 		if (strcasecmp(argv[2], "off") == 0) {
42009c86:	3c0745b7          	lui	a1,0x3c074
42009c8a:	7fc58593          	add	a1,a1,2044 # 3c0747fc <_flash_rodata_start+0x46dc>
42009c8e:	854a                	mv	a0,s2
42009c90:	fdff6097          	auipc	ra,0xfdff6
42009c94:	740080e7          	jalr	1856(ra) # 400003d0 <strcasecmp>
42009c98:	842a                	mv	s0,a0
42009c9a:	cd05                	beqz	a0,42009cd2 <cmd_led+0x1d2>
# 		for (size_t i = 0; i < sizeof(colors) / sizeof(colors[0]); i++) {
42009c9c:	4481                	li	s1,0
42009c9e:	479d                	li	a5,7
42009ca0:	0697e063          	bltu	a5,s1,42009d00 <cmd_led+0x200>
# 			if (strcasecmp(argv[2], colors[i].name) == 0) {
42009ca4:	3c07e7b7          	lui	a5,0x3c07e
42009ca8:	00349713          	sll	a4,s1,0x3
42009cac:	c9478793          	add	a5,a5,-876 # 3c07dc94 <colors>
42009cb0:	97ba                	add	a5,a5,a4
42009cb2:	438c                	lw	a1,0(a5)
42009cb4:	854a                	mv	a0,s2
42009cb6:	fdff6097          	auipc	ra,0xfdff6
42009cba:	71a080e7          	jalr	1818(ra) # 400003d0 <strcasecmp>
42009cbe:	842a                	mv	s0,a0
42009cc0:	cd19                	beqz	a0,42009cde <cmd_led+0x1de>
# 		for (size_t i = 0; i < sizeof(colors) / sizeof(colors[0]); i++) {
42009cc2:	0485                	add	s1,s1,1
42009cc4:	bfe9                	j	42009c9e <cmd_led+0x19e>
# 			leds_set_on(idx, true);
42009cc6:	4585                	li	a1,1
42009cc8:	00f14503          	lbu	a0,15(sp)
42009ccc:	b37ff0ef          	jal	42009802 <leds_set_on>
# 			return 0;
42009cd0:	b54d                	j	42009b72 <cmd_led+0x72>
# 			leds_set_on(idx, false);
42009cd2:	4581                	li	a1,0
42009cd4:	00f14503          	lbu	a0,15(sp)
42009cd8:	b2bff0ef          	jal	42009802 <leds_set_on>
# 			return 0;
42009cdc:	bd59                	j	42009b72 <cmd_led+0x72>
# 				leds_set_color(idx, colors[i].r, colors[i].g, colors[i].b);
42009cde:	3c07e7b7          	lui	a5,0x3c07e
42009ce2:	048e                	sll	s1,s1,0x3
42009ce4:	c9478793          	add	a5,a5,-876 # 3c07dc94 <colors>
42009ce8:	97a6                	add	a5,a5,s1
42009cea:	0067c683          	lbu	a3,6(a5)
42009cee:	0057c603          	lbu	a2,5(a5)
42009cf2:	0047c583          	lbu	a1,4(a5)
42009cf6:	00f14503          	lbu	a0,15(sp)
42009cfa:	b91ff0ef          	jal	4200988a <leds_set_color>
# 				return 0;
42009cfe:	bd95                	j	42009b72 <cmd_led+0x72>
# 		return usage();
42009d00:	c66ff0ef          	jal	42009166 <usage>
42009d04:	842a                	mv	s0,a0
42009d06:	b5b5                	j	42009b72 <cmd_led+0x72>
# 	if (argc == 5 && parse_byte(argv[2], &r) && parse_byte(argv[3], &g) && parse_byte(argv[4], &b)) {
42009d08:	00e10593          	add	a1,sp,14
42009d0c:	00892503          	lw	a0,8(s2)
42009d10:	8ebff0ef          	jal	420095fa <parse_byte>
42009d14:	d521                	beqz	a0,42009c5c <cmd_led+0x15c>
42009d16:	00d10593          	add	a1,sp,13
42009d1a:	00c92503          	lw	a0,12(s2)
42009d1e:	8ddff0ef          	jal	420095fa <parse_byte>
42009d22:	dd0d                	beqz	a0,42009c5c <cmd_led+0x15c>
42009d24:	006c                	add	a1,sp,12
42009d26:	01092503          	lw	a0,16(s2)
42009d2a:	8d1ff0ef          	jal	420095fa <parse_byte>
42009d2e:	d51d                	beqz	a0,42009c5c <cmd_led+0x15c>
# 		leds_set_color(idx, r, g, b);
42009d30:	00c14683          	lbu	a3,12(sp)
42009d34:	00d14603          	lbu	a2,13(sp)
42009d38:	00e14583          	lbu	a1,14(sp)
42009d3c:	00f14503          	lbu	a0,15(sp)
42009d40:	b4bff0ef          	jal	4200988a <leds_set_color>
# 		return 0;
42009d44:	4401                	li	s0,0
42009d46:	b535                	j	42009b72 <cmd_led+0x72>

42009d48 <leds_set_change_cb>:
# 	change_cb = cb;
42009d48:	3fc997b7          	lui	a5,0x3fc99
42009d4c:	38a7a223          	sw	a0,900(a5) # 3fc99384 <change_cb>
# }
42009d50:	8082                	ret

42009d52 <leds_register_cli_commands>:
# 
# void leds_register_cli_commands(void)
# {
42009d52:	7179                	add	sp,sp,-48
42009d54:	d606                	sw	ra,44(sp)
# 	const esp_console_cmd_t led_cmd = {
42009d56:	c202                	sw	zero,4(sp)
42009d58:	c402                	sw	zero,8(sp)
42009d5a:	c602                	sw	zero,12(sp)
42009d5c:	c802                	sw	zero,16(sp)
42009d5e:	ca02                	sw	zero,20(sp)
42009d60:	cc02                	sw	zero,24(sp)
42009d62:	ce02                	sw	zero,28(sp)
42009d64:	3c0757b7          	lui	a5,0x3c075
42009d68:	63878793          	add	a5,a5,1592 # 3c075638 <_flash_rodata_start+0x5518>
42009d6c:	c23e                	sw	a5,4(sp)
42009d6e:	3c0757b7          	lui	a5,0x3c075
42009d72:	63c78793          	add	a5,a5,1596 # 3c07563c <_flash_rodata_start+0x551c>
42009d76:	c43e                	sw	a5,8(sp)
42009d78:	4200a7b7          	lui	a5,0x4200a
42009d7c:	b0078793          	add	a5,a5,-1280 # 42009b00 <cmd_led>
42009d80:	c83e                	sw	a5,16(sp)
# 		.command = "led",
# 		.help = "Addressable LED chain: 'led' for state, 'led help' for usage",
# 		.func = &cmd_led,
# 	};
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&led_cmd));
42009d82:	0048                	add	a0,sp,4
42009d84:	2c0d                	jal	42009fb6 <esp_console_cmd_register>
42009d86:	e501                	bnez	a0,42009d8e <leds_register_cli_commands+0x3c>
# }
42009d88:	50b2                	lw	ra,44(sp)
42009d8a:	6145                	add	sp,sp,48
42009d8c:	8082                	ret
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&led_cmd));
42009d8e:	3c075737          	lui	a4,0x3c075
42009d92:	67c70713          	add	a4,a4,1660 # 3c07567c <_flash_rodata_start+0x555c>
42009d96:	3c07e6b7          	lui	a3,0x3c07e
42009d9a:	c6c68693          	add	a3,a3,-916 # 3c07dc6c <__func__.4>
42009d9e:	22700613          	li	a2,551
42009da2:	3c0755b7          	lui	a1,0x3c075
42009da6:	2f058593          	add	a1,a1,752 # 3c0752f0 <_flash_rodata_start+0x51d0>
42009daa:	fe383097          	auipc	ra,0xfe383
42009dae:	c44080e7          	jalr	-956(ra) # 4038c9ee <_esp_error_check_failed>

