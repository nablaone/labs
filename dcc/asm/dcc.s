# dcc.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

40381b18 <pkt_reset>:
#  * so a sync loss inside it is expected rather than an error. */
# static bool after_packet;
# 
# static void IRAM_ATTR pkt_reset(void)
# {
# 	pkt_state = PKT_PREAMBLE;
40381b18:	3fc997b7          	lui	a5,0x3fc99
40381b1c:	3407aa23          	sw	zero,852(a5) # 3fc99354 <pkt_state>
# 	preamble_ones = 0;
40381b20:	3fc997b7          	lui	a5,0x3fc99
40381b24:	3407a823          	sw	zero,848(a5) # 3fc99350 <preamble_ones>
# }
40381b28:	8082                	ret

40381b2a <on_bit>:
# 		stats.queue_full++;
# 	}
# }
# 
# static void IRAM_ATTR on_bit(int bit)
# {
40381b2a:	1141                	add	sp,sp,-16
40381b2c:	c606                	sw	ra,12(sp)
# 	switch (pkt_state) {
40381b2e:	3fc997b7          	lui	a5,0x3fc99
40381b32:	3547a783          	lw	a5,852(a5) # 3fc99354 <pkt_state>
40381b36:	4705                	li	a4,1
40381b38:	06e78663          	beq	a5,a4,40381ba4 <on_bit+0x7a>
40381b3c:	4709                	li	a4,2
40381b3e:	0ee78463          	beq	a5,a4,40381c26 <on_bit+0xfc>
40381b42:	e7fd                	bnez	a5,40381c30 <on_bit+0x106>
# 	case PKT_PREAMBLE:
# 		if (bit) {
40381b44:	c105                	beqz	a0,40381b64 <on_bit+0x3a>
# 			if (++preamble_ones >= 2) {
40381b46:	3fc99737          	lui	a4,0x3fc99
40381b4a:	35072783          	lw	a5,848(a4) # 3fc99350 <preamble_ones>
40381b4e:	0785                	add	a5,a5,1
40381b50:	34f72823          	sw	a5,848(a4)
40381b54:	4705                	li	a4,1
40381b56:	0cf77d63          	bgeu	a4,a5,40381c30 <on_bit+0x106>
# 				after_packet = false;
40381b5a:	3fc997b7          	lui	a5,0x3fc99
40381b5e:	34078623          	sb	zero,844(a5) # 3fc9934c <after_packet>
40381b62:	a0f9                	j	40381c30 <on_bit+0x106>
# 			}
# 		} else if (preamble_ones >= DCC_PREAMBLE_MIN) {
40381b64:	3fc997b7          	lui	a5,0x3fc99
40381b68:	3507a703          	lw	a4,848(a5) # 3fc99350 <preamble_ones>
40381b6c:	47a5                	li	a5,9
40381b6e:	02e7f663          	bgeu	a5,a4,40381b9a <on_bit+0x70>
# 			/* packet start bit */
# 			pkt_state = PKT_BYTE;
40381b72:	3fc997b7          	lui	a5,0x3fc99
40381b76:	4705                	li	a4,1
40381b78:	34e7aa23          	sw	a4,852(a5) # 3fc99354 <pkt_state>
# 			pkt.len = 0;
40381b7c:	3fc997b7          	lui	a5,0x3fc99
40381b80:	93878793          	add	a5,a5,-1736 # 3fc98938 <pkt>
40381b84:	00078423          	sb	zero,8(a5)
# 			cur_byte = 0;
40381b88:	3fc997b7          	lui	a5,0x3fc99
40381b8c:	34078723          	sb	zero,846(a5) # 3fc9934e <cur_byte>
# 			bit_count = 0;
40381b90:	3fc997b7          	lui	a5,0x3fc99
40381b94:	340786a3          	sb	zero,845(a5) # 3fc9934d <bit_count>
40381b98:	a861                	j	40381c30 <on_bit+0x106>
# 		} else {
# 			preamble_ones = 0;
40381b9a:	3fc997b7          	lui	a5,0x3fc99
40381b9e:	3407a823          	sw	zero,848(a5) # 3fc99350 <preamble_ones>
40381ba2:	a079                	j	40381c30 <on_bit+0x106>
# 		}
# 		break;
# 
# 	case PKT_BYTE:
# 		cur_byte = (uint8_t)((cur_byte << 1) | bit);  /* MSB first */
40381ba4:	3fc99737          	lui	a4,0x3fc99
40381ba8:	34e74783          	lbu	a5,846(a4) # 3fc9934e <cur_byte>
40381bac:	0786                	sll	a5,a5,0x1
40381bae:	8fc9                	or	a5,a5,a0
40381bb0:	0ff7f793          	zext.b	a5,a5
40381bb4:	34f70723          	sb	a5,846(a4)
# 		if (++bit_count == 8) {
40381bb8:	3fc996b7          	lui	a3,0x3fc99
40381bbc:	34d6c703          	lbu	a4,845(a3) # 3fc9934d <bit_count>
40381bc0:	0705                	add	a4,a4,1
40381bc2:	0ff77713          	zext.b	a4,a4
40381bc6:	34e686a3          	sb	a4,845(a3)
40381bca:	46a1                	li	a3,8
40381bcc:	06d71263          	bne	a4,a3,40381c30 <on_bit+0x106>
# 			if (pkt.len == DCC_MAX_BYTES) {
40381bd0:	3fc99737          	lui	a4,0x3fc99
40381bd4:	93870713          	add	a4,a4,-1736 # 3fc98938 <pkt>
40381bd8:	00874683          	lbu	a3,8(a4)
40381bdc:	4721                	li	a4,8
40381bde:	02e68b63          	beq	a3,a4,40381c14 <on_bit+0xea>
# 				stats.overflow++;
# 				pkt_reset();
# 			} else {
# 				pkt.data[pkt.len++] = cur_byte;
40381be2:	3fc99737          	lui	a4,0x3fc99
40381be6:	93870713          	add	a4,a4,-1736 # 3fc98938 <pkt>
40381bea:	00168613          	add	a2,a3,1
40381bee:	00c70423          	sb	a2,8(a4)
40381bf2:	9736                	add	a4,a4,a3
40381bf4:	00f704a3          	sb	a5,9(a4)
# 				pkt_state = PKT_SEPARATOR;
40381bf8:	3fc997b7          	lui	a5,0x3fc99
40381bfc:	4709                	li	a4,2
40381bfe:	34e7aa23          	sw	a4,852(a5) # 3fc99354 <pkt_state>
# 			}
# 			cur_byte = 0;
40381c02:	3fc997b7          	lui	a5,0x3fc99
40381c06:	34078723          	sb	zero,846(a5) # 3fc9934e <cur_byte>
# 			bit_count = 0;
40381c0a:	3fc997b7          	lui	a5,0x3fc99
40381c0e:	340786a3          	sb	zero,845(a5) # 3fc9934d <bit_count>
40381c12:	a839                	j	40381c30 <on_bit+0x106>
# 				stats.overflow++;
40381c14:	3fc997b7          	lui	a5,0x3fc99
40381c18:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381c1c:	4fd8                	lw	a4,28(a5)
40381c1e:	0705                	add	a4,a4,1
40381c20:	cfd8                	sw	a4,28(a5)
# 				pkt_reset();
40381c22:	3ddd                	jal	40381b18 <pkt_reset>
40381c24:	bff9                	j	40381c02 <on_bit+0xd8>
# 		break;
# 
# 	case PKT_SEPARATOR:
# 		/* A "0" is a data byte start bit. A "1" (end bit) never gets
# 		 * here -- dcc_edge_isr() handles it on its first half. */
# 		pkt_state = PKT_BYTE;
40381c26:	3fc997b7          	lui	a5,0x3fc99
40381c2a:	4705                	li	a4,1
40381c2c:	34e7aa23          	sw	a4,852(a5) # 3fc99354 <pkt_state>
# 		break;
# 	}
# }
40381c30:	40b2                	lw	ra,12(sp)
40381c32:	0141                	add	sp,sp,16
40381c34:	8082                	ret

40381c36 <lose_sync>:
# 
# /* Drops back to hunting for a preamble. Errors while synced are counted
#  * here, either as the given stat or as a cutout (see after_packet). */
# static void IRAM_ATTR lose_sync(volatile uint32_t *stat)
# {
40381c36:	1141                	add	sp,sp,-16
40381c38:	c606                	sw	ra,12(sp)
# 	if (synced) {
40381c3a:	3fc997b7          	lui	a5,0x3fc99
40381c3e:	35c7c783          	lbu	a5,860(a5) # 3fc9935c <synced>
40381c42:	cf89                	beqz	a5,40381c5c <lose_sync+0x26>
# 		(*(after_packet ? &stats.cutouts : stat))++;
40381c44:	3fc997b7          	lui	a5,0x3fc99
40381c48:	34c7c783          	lbu	a5,844(a5) # 3fc9934c <after_packet>
40381c4c:	c789                	beqz	a5,40381c56 <lose_sync+0x20>
40381c4e:	3fc99537          	lui	a0,0x3fc99
40381c52:	9b450513          	add	a0,a0,-1612 # 3fc989b4 <stats+0x10>
40381c56:	411c                	lw	a5,0(a0)
40381c58:	0785                	add	a5,a5,1
40381c5a:	c11c                	sw	a5,0(a0)
# 	}
# 	after_packet = false;
40381c5c:	3fc997b7          	lui	a5,0x3fc99
40381c60:	34078623          	sb	zero,844(a5) # 3fc9934c <after_packet>
# 	synced = false;
40381c64:	3fc997b7          	lui	a5,0x3fc99
40381c68:	34078e23          	sb	zero,860(a5) # 3fc9935c <synced>
# 	one_halves = 0;
40381c6c:	3fc997b7          	lui	a5,0x3fc99
40381c70:	3407ac23          	sw	zero,856(a5) # 3fc99358 <one_halves>
# 	pending_half = -1;
40381c74:	577d                	li	a4,-1
40381c76:	84e1a423          	sw	a4,-1976(gp) # 3fc95248 <pending_half>
# 	pkt_reset();
40381c7a:	3d79                	jal	40381b18 <pkt_reset>
# }
40381c7c:	40b2                	lw	ra,12(sp)
40381c7e:	0141                	add	sp,sp,16
40381c80:	8082                	ret

40381c82 <pkt_end>:
# {
40381c82:	1141                	add	sp,sp,-16
40381c84:	c606                	sw	ra,12(sp)
# 	if (pkt.len < 3) {
40381c86:	3fc997b7          	lui	a5,0x3fc99
40381c8a:	93878793          	add	a5,a5,-1736 # 3fc98938 <pkt>
40381c8e:	0087c803          	lbu	a6,8(a5)
40381c92:	4789                	li	a5,2
40381c94:	0107f563          	bgeu	a5,a6,40381c9e <pkt_end+0x1c>
# 	for (int i = 0; i < pkt.len; i++) {
40381c98:	4701                	li	a4,0
# 	uint8_t x = 0;
40381c9a:	4681                	li	a3,0
40381c9c:	a025                	j	40381cc4 <pkt_end+0x42>
# 		stats.too_short++;
40381c9e:	3fc997b7          	lui	a5,0x3fc99
40381ca2:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381ca6:	4f98                	lw	a4,24(a5)
40381ca8:	0705                	add	a4,a4,1
40381caa:	cf98                	sw	a4,24(a5)
# }
40381cac:	40b2                	lw	ra,12(sp)
40381cae:	0141                	add	sp,sp,16
40381cb0:	8082                	ret
# 		x ^= pkt.data[i];
40381cb2:	3fc997b7          	lui	a5,0x3fc99
40381cb6:	93878793          	add	a5,a5,-1736 # 3fc98938 <pkt>
40381cba:	97ba                	add	a5,a5,a4
40381cbc:	0097c783          	lbu	a5,9(a5)
40381cc0:	8ebd                	xor	a3,a3,a5
# 	for (int i = 0; i < pkt.len; i++) {
40381cc2:	0705                	add	a4,a4,1
40381cc4:	ff0747e3          	blt	a4,a6,40381cb2 <pkt_end+0x30>
# 	if (x != 0) {
40381cc8:	ca89                	beqz	a3,40381cda <pkt_end+0x58>
# 		stats.bad_xor++;
40381cca:	3fc997b7          	lui	a5,0x3fc99
40381cce:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381cd2:	4bd8                	lw	a4,20(a5)
40381cd4:	0705                	add	a4,a4,1
40381cd6:	cbd8                	sw	a4,20(a5)
# 		return;
40381cd8:	bfd1                	j	40381cac <pkt_end+0x2a>
# 	stats.packets_ok++;
40381cda:	3fc997b7          	lui	a5,0x3fc99
40381cde:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381ce2:	43d8                	lw	a4,4(a5)
40381ce4:	0705                	add	a4,a4,1
40381ce6:	c3d8                	sw	a4,4(a5)
# 	pkt.time_us = now;
40381ce8:	3fc997b7          	lui	a5,0x3fc99
40381cec:	92a7ac23          	sw	a0,-1736(a5) # 3fc98938 <pkt>
40381cf0:	92b7ae23          	sw	a1,-1732(a5)
# 	if (xQueueSendFromISR(dcc_rx_queue, &pkt, woken) != pdTRUE) {
40381cf4:	4681                	li	a3,0
40381cf6:	93878593          	add	a1,a5,-1736
40381cfa:	3fc997b7          	lui	a5,0x3fc99
40381cfe:	3687a503          	lw	a0,872(a5) # 3fc99368 <dcc_rx_queue>
40381d02:	5560b0ef          	jal	4038d258 <xQueueGenericSendFromISR>
40381d06:	4785                	li	a5,1
40381d08:	faf502e3          	beq	a0,a5,40381cac <pkt_end+0x2a>
# 		stats.queue_full++;
40381d0c:	3fc997b7          	lui	a5,0x3fc99
40381d10:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381d14:	5398                	lw	a4,32(a5)
40381d16:	0705                	add	a4,a4,1
40381d18:	d398                	sw	a4,32(a5)
40381d1a:	bf49                	j	40381cac <pkt_end+0x2a>

40381d1c <dcc_edge_isr>:
#  * Fires on every edge of the opto output. The time since the previous
#  * edge is one half-bit, whichever polarity it had -- the decoder never
#  * needs to know which rail is positive.
#  */
# static void IRAM_ATTR dcc_edge_isr(void *arg)
# {
40381d1c:	1101                	add	sp,sp,-32
40381d1e:	ce06                	sw	ra,28(sp)
40381d20:	cc22                	sw	s0,24(sp)
# 	int64_t now = esp_timer_get_time();
40381d22:	e02fe0ef          	jal	40380324 <esp_timer_get_time>
# 	uint32_t dt = (uint32_t)(now - last_edge_us);
40381d26:	3fc997b7          	lui	a5,0x3fc99
40381d2a:	3607a683          	lw	a3,864(a5) # 3fc99360 <last_edge_us>
40381d2e:	40d506b3          	sub	a3,a0,a3
# 	last_edge_us = now;
40381d32:	36a7a023          	sw	a0,864(a5)
40381d36:	36b7a223          	sw	a1,868(a5)
# 	stats.edges++;
40381d3a:	3fc997b7          	lui	a5,0x3fc99
40381d3e:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
40381d42:	4398                	lw	a4,0(a5)
40381d44:	0705                	add	a4,a4,1
40381d46:	c398                	sw	a4,0(a5)
# 
# 	uint32_t bucket = dt / HIST_BUCKET_US;
40381d48:	47a9                	li	a5,10
40381d4a:	02f6d7b3          	divu	a5,a3,a5
# 	hist[bucket < HIST_BUCKETS ? bucket : HIST_BUCKETS - 1]++;
40381d4e:	4751                	li	a4,20
40381d50:	00f77363          	bgeu	a4,a5,40381d56 <dcc_edge_isr+0x3a>
40381d54:	47d1                	li	a5,20
40381d56:	3fc99737          	lui	a4,0x3fc99
40381d5a:	078a                	sll	a5,a5,0x2
40381d5c:	95070713          	add	a4,a4,-1712 # 3fc98950 <hist>
40381d60:	97ba                	add	a5,a5,a4
40381d62:	4398                	lw	a4,0(a5)
40381d64:	0705                	add	a4,a4,1
40381d66:	c398                	sw	a4,0(a5)
# 
# 	int half;
# 	if (dt >= HALF_ONE_MIN_US && dt <= HALF_ONE_MAX_US) {
40381d68:	fdd68793          	add	a5,a3,-35
40381d6c:	02a00713          	li	a4,42
40381d70:	02f77163          	bgeu	a4,a5,40381d92 <dcc_edge_isr+0x76>
# 		half = 1;
# 	} else if (dt > HALF_ONE_MAX_US && dt <= HALF_ZERO_MAX_US) {
40381d74:	fb268693          	add	a3,a3,-78
40381d78:	678d                	lui	a5,0x3
40381d7a:	e9278793          	add	a5,a5,-366 # 2e92 <_rtc_fast_length+0x2c92>
40381d7e:	00d7e463          	bltu	a5,a3,40381d86 <dcc_edge_isr+0x6a>
# 		half = 0;
40381d82:	4401                	li	s0,0
40381d84:	a801                	j	40381d94 <dcc_edge_isr+0x78>
# 	} else {
# 		lose_sync(&stats.bad_timing);
40381d86:	3fc99537          	lui	a0,0x3fc99
40381d8a:	9ac50513          	add	a0,a0,-1620 # 3fc989ac <stats+0x8>
40381d8e:	3565                	jal	40381c36 <lose_sync>
# 		return;
40381d90:	a8b5                	j	40381e0c <dcc_edge_isr+0xf0>
# 		half = 1;
40381d92:	4405                	li	s0,1
# 	}
# 
# 	if (!synced) {
40381d94:	3fc997b7          	lui	a5,0x3fc99
40381d98:	35c7c783          	lbu	a5,860(a5) # 3fc9935c <synced>
40381d9c:	e7b1                	bnez	a5,40381de8 <dcc_edge_isr+0xcc>
# 		if (half) {
40381d9e:	c809                	beqz	s0,40381db0 <dcc_edge_isr+0x94>
# 			one_halves++;
40381da0:	3fc99737          	lui	a4,0x3fc99
40381da4:	35872783          	lw	a5,856(a4) # 3fc99358 <one_halves>
40381da8:	0785                	add	a5,a5,1
40381daa:	34f72c23          	sw	a5,856(a4)
# 			return;
40381dae:	a8b9                	j	40381e0c <dcc_edge_isr+0xf0>
# 		}
# 		if (one_halves >= 2 * DCC_PREAMBLE_MIN) {
40381db0:	3fc997b7          	lui	a5,0x3fc99
40381db4:	3587a783          	lw	a5,856(a5) # 3fc99358 <one_halves>
40381db8:	474d                	li	a4,19
40381dba:	02f77263          	bgeu	a4,a5,40381dde <dcc_edge_isr+0xc2>
# 			/* first half of the start bit */
# 			synced = true;
40381dbe:	3fc99737          	lui	a4,0x3fc99
40381dc2:	4685                	li	a3,1
40381dc4:	34d70e23          	sb	a3,860(a4) # 3fc9935c <synced>
# 			pkt_state = PKT_PREAMBLE;
40381dc8:	3fc99737          	lui	a4,0x3fc99
40381dcc:	34072a23          	sw	zero,852(a4) # 3fc99354 <pkt_state>
# 			preamble_ones = one_halves / 2;
40381dd0:	8385                	srl	a5,a5,0x1
40381dd2:	3fc99737          	lui	a4,0x3fc99
40381dd6:	34f72823          	sw	a5,848(a4) # 3fc99350 <preamble_ones>
# 			pending_half = 0;
40381dda:	8401a423          	sw	zero,-1976(gp) # 3fc95248 <pending_half>
# 		}
# 		one_halves = 0;
40381dde:	3fc997b7          	lui	a5,0x3fc99
40381de2:	3407ac23          	sw	zero,856(a5) # 3fc99358 <one_halves>
# 		return;
40381de6:	a01d                	j	40381e0c <dcc_edge_isr+0xf0>
# 	}
# 
# 	BaseType_t woken = pdFALSE;
40381de8:	c602                	sw	zero,12(sp)
# 
# 	if (pending_half < 0) {
40381dea:	8481a783          	lw	a5,-1976(gp) # 3fc95248 <pending_half>
40381dee:	0207c363          	bltz	a5,40381e14 <dcc_edge_isr+0xf8>
# 			/* first half of the packet end bit */
# 			pkt_end(now, &woken);
# 			pkt_reset();
# 			after_packet = true;
# 		}
# 	} else if (half != pending_half) {
40381df2:	04878563          	beq	a5,s0,40381e3c <dcc_edge_isr+0x120>
# 		lose_sync(&stats.bad_pair);
40381df6:	3fc99537          	lui	a0,0x3fc99
40381dfa:	9b050513          	add	a0,a0,-1616 # 3fc989b0 <stats+0xc>
40381dfe:	3d25                	jal	40381c36 <lose_sync>
# 		one_halves = half;
40381e00:	3fc997b7          	lui	a5,0x3fc99
40381e04:	3487ac23          	sw	s0,856(a5) # 3fc99358 <one_halves>
# 	} else {
# 		on_bit(half);
# 		pending_half = -1;
# 	}
# 
# 	if (woken) {
40381e08:	47b2                	lw	a5,12(sp)
40381e0a:	e3a1                	bnez	a5,40381e4a <dcc_edge_isr+0x12e>
# 		portYIELD_FROM_ISR();
# 	}
# }
40381e0c:	40f2                	lw	ra,28(sp)
40381e0e:	4462                	lw	s0,24(sp)
40381e10:	6105                	add	sp,sp,32
40381e12:	8082                	ret
# 		pending_half = half;
40381e14:	8481a423          	sw	s0,-1976(gp) # 3fc95248 <pending_half>
# 		if (half && pkt_state == PKT_SEPARATOR) {
40381e18:	d865                	beqz	s0,40381e08 <dcc_edge_isr+0xec>
40381e1a:	3fc997b7          	lui	a5,0x3fc99
40381e1e:	3547a703          	lw	a4,852(a5) # 3fc99354 <pkt_state>
40381e22:	4789                	li	a5,2
40381e24:	fef712e3          	bne	a4,a5,40381e08 <dcc_edge_isr+0xec>
# 			pkt_end(now, &woken);
40381e28:	0070                	add	a2,sp,12
40381e2a:	3da1                	jal	40381c82 <pkt_end>
# 			pkt_reset();
40381e2c:	cedff0ef          	jal	40381b18 <pkt_reset>
# 			after_packet = true;
40381e30:	3fc997b7          	lui	a5,0x3fc99
40381e34:	4705                	li	a4,1
40381e36:	34e78623          	sb	a4,844(a5) # 3fc9934c <after_packet>
40381e3a:	b7f9                	j	40381e08 <dcc_edge_isr+0xec>
# 		on_bit(half);
40381e3c:	8522                	mv	a0,s0
40381e3e:	cedff0ef          	jal	40381b2a <on_bit>
# 		pending_half = -1;
40381e42:	577d                	li	a4,-1
40381e44:	84e1a423          	sw	a4,-1976(gp) # 3fc95248 <pending_half>
40381e48:	b7c1                	j	40381e08 <dcc_edge_isr+0xec>
# 		portYIELD_FROM_ISR();
40381e4a:	6b10b0ef          	jal	4038dcfa <vPortYieldFromISR>
40381e4e:	bf7d                	j	40381e0c <dcc_edge_isr+0xf0>

42007ba8 <is_idle>:
# 
# /* ---- CLI ---- */
# 
# static bool is_idle(const dcc_packet_t *p)
# {
# 	return p->len == 3 && p->data[0] == 0xFF && p->data[1] == 0x00;
42007ba8:	00855703          	lhu	a4,8(a0)
42007bac:	67c1                	lui	a5,0x10
42007bae:	f0378793          	add	a5,a5,-253 # ff03 <_rtc_fast_length+0xfd03>
42007bb2:	00f70463          	beq	a4,a5,42007bba <is_idle+0x12>
42007bb6:	4501                	li	a0,0
42007bb8:	8082                	ret
42007bba:	00a54783          	lbu	a5,10(a0)
42007bbe:	c399                	beqz	a5,42007bc4 <is_idle+0x1c>
42007bc0:	4501                	li	a0,0
42007bc2:	8082                	ret
42007bc4:	4505                	li	a0,1
# }
42007bc6:	8082                	ret

42007bc8 <describe_accessory>:
# {
42007bc8:	1141                	add	sp,sp,-16
42007bca:	c606                	sw	ra,12(sp)
42007bcc:	882a                	mv	a6,a0
42007bce:	832e                	mv	t1,a1
42007bd0:	8532                	mv	a0,a2
42007bd2:	85b6                	mv	a1,a3
# 	if (n < 2) {
42007bd4:	4785                	li	a5,1
42007bd6:	0667d363          	bge	a5,t1,42007c3c <describe_accessory+0x74>
# 	uint8_t b1 = d[0];
42007bda:	00084783          	lbu	a5,0(a6)
# 	uint8_t b2 = d[1];
42007bde:	00184703          	lbu	a4,1(a6)
# 	unsigned wire = ((unsigned)((~b2 >> 4) & 0x07) << 8) |
42007be2:	fff74893          	not	a7,a4
42007be6:	4048d893          	sra	a7,a7,0x4
42007bea:	08a2                	sll	a7,a7,0x8
42007bec:	7008f893          	and	a7,a7,1792
# 			((unsigned)(b1 & 0x3F) << 2) |
42007bf0:	078a                	sll	a5,a5,0x2
42007bf2:	0ff7f793          	zext.b	a5,a5
# 	unsigned wire = ((unsigned)((~b2 >> 4) & 0x07) << 8) |
42007bf6:	00f8e8b3          	or	a7,a7,a5
# 			((b2 >> 1) & 0x03);
42007bfa:	00175793          	srl	a5,a4,0x1
42007bfe:	8b8d                	and	a5,a5,3
# 	unsigned wire = ((unsigned)((~b2 >> 4) & 0x07) << 8) |
42007c00:	00f8e8b3          	or	a7,a7,a5
# 	unsigned user = wire >= 4 ? wire - 3 : wire + 2045;
42007c04:	478d                	li	a5,3
42007c06:	0517f263          	bgeu	a5,a7,42007c4a <describe_accessory+0x82>
42007c0a:	ffd88693          	add	a3,a7,-3
# 	if (b2 & 0x80) {
42007c0e:	01871793          	sll	a5,a4,0x18
42007c12:	87e1                	sra	a5,a5,0x18
42007c14:	0207ce63          	bltz	a5,42007c50 <describe_accessory+0x88>
# 	} else if ((b2 & 0x09) == 0x01 && n >= 3) {
42007c18:	00977793          	and	a5,a4,9
42007c1c:	4605                	li	a2,1
42007c1e:	08c78363          	beq	a5,a2,42007ca4 <describe_accessory+0xdc>
# 	} else if (b2 & 0x08) {
42007c22:	00877793          	and	a5,a4,8
42007c26:	cfc1                	beqz	a5,42007cbe <describe_accessory+0xf6>
# 		snprintf(buf, size, "ACC NOP %u (wire %u)", user, wire);
42007c28:	8746                	mv	a4,a7
42007c2a:	3c075637          	lui	a2,0x3c075
42007c2e:	87060613          	add	a2,a2,-1936 # 3c074870 <_flash_rodata_start+0x4750>
42007c32:	03d530ef          	jal	4205b46e <snprintf>
# }
42007c36:	40b2                	lw	ra,12(sp)
42007c38:	0141                	add	sp,sp,16
42007c3a:	8082                	ret
# 		snprintf(buf, size, "ACC (truncated)");
42007c3c:	3c075637          	lui	a2,0x3c075
42007c40:	80060613          	add	a2,a2,-2048 # 3c074800 <_flash_rodata_start+0x46e0>
42007c44:	02b530ef          	jal	4205b46e <snprintf>
# 		return;
42007c48:	b7fd                	j	42007c36 <describe_accessory+0x6e>
# 	unsigned user = wire >= 4 ? wire - 3 : wire + 2045;
42007c4a:	7fd88693          	add	a3,a7,2045
42007c4e:	b7c1                	j	42007c0e <describe_accessory+0x46>
# 		if (n == 2) {
42007c50:	4789                	li	a5,2
42007c52:	04f31163          	bne	t1,a5,42007c94 <describe_accessory+0xcc>
# 			int r = b2 & 0x01;
42007c56:	00177613          	and	a2,a4,1
# 			snprintf(buf, size, "ACC %u R=%d %s %s (wire %u)", user, r,
42007c5a:	c21d                	beqz	a2,42007c80 <describe_accessory+0xb8>
42007c5c:	3c0747b7          	lui	a5,0x3c074
42007c60:	7d878793          	add	a5,a5,2008 # 3c0747d8 <_flash_rodata_start+0x46b8>
42007c64:	8b21                	and	a4,a4,8
42007c66:	c315                	beqz	a4,42007c8a <describe_accessory+0xc2>
42007c68:	3c074837          	lui	a6,0x3c074
42007c6c:	7f880813          	add	a6,a6,2040 # 3c0747f8 <_flash_rodata_start+0x46d8>
42007c70:	8732                	mv	a4,a2
42007c72:	3c075637          	lui	a2,0x3c075
42007c76:	81060613          	add	a2,a2,-2032 # 3c074810 <_flash_rodata_start+0x46f0>
42007c7a:	7f4530ef          	jal	4205b46e <snprintf>
42007c7e:	bf65                	j	42007c36 <describe_accessory+0x6e>
42007c80:	3c0747b7          	lui	a5,0x3c074
42007c84:	7e878793          	add	a5,a5,2024 # 3c0747e8 <_flash_rodata_start+0x46c8>
42007c88:	bff1                	j	42007c64 <describe_accessory+0x9c>
42007c8a:	3c074837          	lui	a6,0x3c074
42007c8e:	7fc80813          	add	a6,a6,2044 # 3c0747fc <_flash_rodata_start+0x46dc>
42007c92:	bff9                	j	42007c70 <describe_accessory+0xa8>
# 			snprintf(buf, size, "ACC %u CV access (POM) (wire %u)", user, wire);
42007c94:	8746                	mv	a4,a7
42007c96:	3c075637          	lui	a2,0x3c075
42007c9a:	82c60613          	add	a2,a2,-2004 # 3c07482c <_flash_rodata_start+0x470c>
42007c9e:	7d0530ef          	jal	4205b46e <snprintf>
42007ca2:	bf51                	j	42007c36 <describe_accessory+0x6e>
# 	} else if ((b2 & 0x09) == 0x01 && n >= 3) {
42007ca4:	4789                	li	a5,2
42007ca6:	f667dee3          	bge	a5,t1,42007c22 <describe_accessory+0x5a>
# 		snprintf(buf, size, "ACC-EXT %u value %u (wire %u)", user, d[2], wire);
42007caa:	87c6                	mv	a5,a7
42007cac:	00284703          	lbu	a4,2(a6)
42007cb0:	3c075637          	lui	a2,0x3c075
42007cb4:	85060613          	add	a2,a2,-1968 # 3c074850 <_flash_rodata_start+0x4730>
42007cb8:	7b6530ef          	jal	4205b46e <snprintf>
42007cbc:	bfad                	j	42007c36 <describe_accessory+0x6e>
# 		snprintf(buf, size, "ACC %u unknown 0x%02X (wire %u)", user, b2, wire);
42007cbe:	87c6                	mv	a5,a7
42007cc0:	3c075637          	lui	a2,0x3c075
42007cc4:	88860613          	add	a2,a2,-1912 # 3c074888 <_flash_rodata_start+0x4768>
42007cc8:	7a6530ef          	jal	4205b46e <snprintf>
42007ccc:	b7ad                	j	42007c36 <describe_accessory+0x6e>

42007cce <describe_loco_instr>:
# {
42007cce:	1141                	add	sp,sp,-16
42007cd0:	c606                	sw	ra,12(sp)
42007cd2:	872a                	mv	a4,a0
42007cd4:	87ae                	mv	a5,a1
42007cd6:	8532                	mv	a0,a2
42007cd8:	85b6                	mv	a1,a3
# 	if (n < 1) {
42007cda:	06f05563          	blez	a5,42007d44 <describe_loco_instr+0x76>
# 	uint8_t b = p[0];
42007cde:	00074303          	lbu	t1,0(a4)
# 	if (b == 0x3F && n >= 2) {
42007ce2:	03f00693          	li	a3,63
42007ce6:	06d30863          	beq	t1,a3,42007d56 <describe_loco_instr+0x88>
# 	} else if ((b & 0xC0) == 0x40) {
42007cea:	0c037813          	and	a6,t1,192
42007cee:	04000693          	li	a3,64
42007cf2:	0cd80363          	beq	a6,a3,42007db8 <describe_loco_instr+0xea>
# 	} else if ((b & 0xE0) == 0x80) {
42007cf6:	0e037693          	and	a3,t1,224
42007cfa:	08000613          	li	a2,128
42007cfe:	12c68063          	beq	a3,a2,42007e1e <describe_loco_instr+0x150>
# 	} else if ((b & 0xF0) == 0xB0) {
42007d02:	0f037693          	and	a3,t1,240
42007d06:	0b000613          	li	a2,176
42007d0a:	14c68163          	beq	a3,a2,42007e4c <describe_loco_instr+0x17e>
# 	} else if ((b & 0xF0) == 0xA0) {
42007d0e:	0a000613          	li	a2,160
42007d12:	16c68063          	beq	a3,a2,42007e72 <describe_loco_instr+0x1a4>
# 	} else if (b == 0xDE && n >= 2) {
42007d16:	0de00613          	li	a2,222
42007d1a:	16c30f63          	beq	t1,a2,42007e98 <describe_loco_instr+0x1ca>
# 	} else if (b == 0xDF && n >= 2) {
42007d1e:	0df00613          	li	a2,223
42007d22:	18c30763          	beq	t1,a2,42007eb0 <describe_loco_instr+0x1e2>
# 	} else if ((b & 0xF0) == 0xE0) {
42007d26:	0e000793          	li	a5,224
42007d2a:	18f68f63          	beq	a3,a5,42007ec8 <describe_loco_instr+0x1fa>
# 	} else if ((b & 0xE0) == 0x00) {
42007d2e:	47fd                	li	a5,31
42007d30:	1a67e363          	bltu	a5,t1,42007ed6 <describe_loco_instr+0x208>
# 		snprintf(buf, size, "decoder/consist control 0x%02X", b);
42007d34:	869a                	mv	a3,t1
42007d36:	3c075637          	lui	a2,0x3c075
42007d3a:	96c60613          	add	a2,a2,-1684 # 3c07496c <_flash_rodata_start+0x484c>
42007d3e:	730530ef          	jal	4205b46e <snprintf>
42007d42:	a039                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "(no instruction)");
42007d44:	3c075637          	lui	a2,0x3c075
42007d48:	8b060613          	add	a2,a2,-1872 # 3c0748b0 <_flash_rodata_start+0x4790>
42007d4c:	722530ef          	jal	4205b46e <snprintf>
# }
42007d50:	40b2                	lw	ra,12(sp)
42007d52:	0141                	add	sp,sp,16
42007d54:	8082                	ret
# 	if (b == 0x3F && n >= 2) {
42007d56:	4685                	li	a3,1
42007d58:	f8f6d9e3          	bge	a3,a5,42007cea <describe_loco_instr+0x1c>
# 		const char *dir = (p[1] & 0x80) ? "fwd" : "rev";
42007d5c:	00174783          	lbu	a5,1(a4)
42007d60:	01879713          	sll	a4,a5,0x18
42007d64:	8761                	sra	a4,a4,0x18
42007d66:	02074663          	bltz	a4,42007d92 <describe_loco_instr+0xc4>
42007d6a:	3c0756b7          	lui	a3,0x3c075
42007d6e:	8ac68693          	add	a3,a3,-1876 # 3c0748ac <_flash_rodata_start+0x478c>
# 		int s = p[1] & 0x7F;
42007d72:	07f7f793          	and	a5,a5,127
# 		if (s == 0) {
42007d76:	c39d                	beqz	a5,42007d9c <describe_loco_instr+0xce>
# 		} else if (s == 1) {
42007d78:	4705                	li	a4,1
42007d7a:	02e78863          	beq	a5,a4,42007daa <describe_loco_instr+0xdc>
# 			snprintf(buf, size, "speed %d/126 %s", s - 1, dir);
42007d7e:	8736                	mv	a4,a3
42007d80:	fff78693          	add	a3,a5,-1
42007d84:	3c075637          	lui	a2,0x3c075
42007d88:	8e460613          	add	a2,a2,-1820 # 3c0748e4 <_flash_rodata_start+0x47c4>
42007d8c:	6e2530ef          	jal	4205b46e <snprintf>
42007d90:	b7c1                	j	42007d50 <describe_loco_instr+0x82>
# 		const char *dir = (p[1] & 0x80) ? "fwd" : "rev";
42007d92:	3c0756b7          	lui	a3,0x3c075
42007d96:	8a868693          	add	a3,a3,-1880 # 3c0748a8 <_flash_rodata_start+0x4788>
42007d9a:	bfe1                	j	42007d72 <describe_loco_instr+0xa4>
# 			snprintf(buf, size, "speed STOP %s", dir);
42007d9c:	3c075637          	lui	a2,0x3c075
42007da0:	8c460613          	add	a2,a2,-1852 # 3c0748c4 <_flash_rodata_start+0x47a4>
42007da4:	6ca530ef          	jal	4205b46e <snprintf>
42007da8:	b765                	j	42007d50 <describe_loco_instr+0x82>
# 			snprintf(buf, size, "speed E-STOP %s", dir);
42007daa:	3c075637          	lui	a2,0x3c075
42007dae:	8d460613          	add	a2,a2,-1836 # 3c0748d4 <_flash_rodata_start+0x47b4>
42007db2:	6bc530ef          	jal	4205b46e <snprintf>
42007db6:	bf69                	j	42007d50 <describe_loco_instr+0x82>
# 		const char *dir = (b & 0x20) ? "fwd" : "rev";
42007db8:	02037793          	and	a5,t1,32
42007dbc:	cb9d                	beqz	a5,42007df2 <describe_loco_instr+0x124>
42007dbe:	3c0756b7          	lui	a3,0x3c075
42007dc2:	8a868693          	add	a3,a3,-1880 # 3c0748a8 <_flash_rodata_start+0x4788>
# 		int v = ((b & 0x0F) << 1) | ((b >> 4) & 1);
42007dc6:	00131793          	sll	a5,t1,0x1
42007dca:	8bf9                	and	a5,a5,30
42007dcc:	00435313          	srl	t1,t1,0x4
42007dd0:	00137313          	and	t1,t1,1
42007dd4:	0067e7b3          	or	a5,a5,t1
# 		if (v <= 1) {
42007dd8:	4705                	li	a4,1
42007dda:	02f75163          	bge	a4,a5,42007dfc <describe_loco_instr+0x12e>
# 		} else if (v <= 3) {
42007dde:	470d                	li	a4,3
42007de0:	02f74563          	blt	a4,a5,42007e0a <describe_loco_instr+0x13c>
# 			snprintf(buf, size, "speed E-STOP %s", dir);
42007de4:	3c075637          	lui	a2,0x3c075
42007de8:	8d460613          	add	a2,a2,-1836 # 3c0748d4 <_flash_rodata_start+0x47b4>
42007dec:	682530ef          	jal	4205b46e <snprintf>
42007df0:	b785                	j	42007d50 <describe_loco_instr+0x82>
# 		const char *dir = (b & 0x20) ? "fwd" : "rev";
42007df2:	3c0756b7          	lui	a3,0x3c075
42007df6:	8ac68693          	add	a3,a3,-1876 # 3c0748ac <_flash_rodata_start+0x478c>
42007dfa:	b7f1                	j	42007dc6 <describe_loco_instr+0xf8>
# 			snprintf(buf, size, "speed STOP %s", dir);
42007dfc:	3c075637          	lui	a2,0x3c075
42007e00:	8c460613          	add	a2,a2,-1852 # 3c0748c4 <_flash_rodata_start+0x47a4>
42007e04:	66a530ef          	jal	4205b46e <snprintf>
42007e08:	b7a1                	j	42007d50 <describe_loco_instr+0x82>
# 			snprintf(buf, size, "speed %d/28 %s", v - 3, dir);
42007e0a:	8736                	mv	a4,a3
42007e0c:	ffd78693          	add	a3,a5,-3
42007e10:	3c075637          	lui	a2,0x3c075
42007e14:	8f460613          	add	a2,a2,-1804 # 3c0748f4 <_flash_rodata_start+0x47d4>
42007e18:	656530ef          	jal	4205b46e <snprintf>
42007e1c:	bf15                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "F0=%d F1-F4=%d%d%d%d", (b >> 4) & 1,
42007e1e:	00435693          	srl	a3,t1,0x4
42007e22:	00135793          	srl	a5,t1,0x1
42007e26:	00235813          	srl	a6,t1,0x2
42007e2a:	00335893          	srl	a7,t1,0x3
42007e2e:	0018f893          	and	a7,a7,1
42007e32:	00187813          	and	a6,a6,1
42007e36:	8b85                	and	a5,a5,1
42007e38:	00137713          	and	a4,t1,1
42007e3c:	8a85                	and	a3,a3,1
42007e3e:	3c075637          	lui	a2,0x3c075
42007e42:	90460613          	add	a2,a2,-1788 # 3c074904 <_flash_rodata_start+0x47e4>
42007e46:	628530ef          	jal	4205b46e <snprintf>
42007e4a:	b719                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "F5-F8=%d%d%d%d",
42007e4c:	00135713          	srl	a4,t1,0x1
42007e50:	00235793          	srl	a5,t1,0x2
42007e54:	00335813          	srl	a6,t1,0x3
42007e58:	00187813          	and	a6,a6,1
42007e5c:	8b85                	and	a5,a5,1
42007e5e:	8b05                	and	a4,a4,1
42007e60:	00137693          	and	a3,t1,1
42007e64:	3c075637          	lui	a2,0x3c075
42007e68:	91c60613          	add	a2,a2,-1764 # 3c07491c <_flash_rodata_start+0x47fc>
42007e6c:	602530ef          	jal	4205b46e <snprintf>
42007e70:	b5c5                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "F9-F12=%d%d%d%d",
42007e72:	00135713          	srl	a4,t1,0x1
42007e76:	00235793          	srl	a5,t1,0x2
42007e7a:	00335813          	srl	a6,t1,0x3
42007e7e:	00187813          	and	a6,a6,1
42007e82:	8b85                	and	a5,a5,1
42007e84:	8b05                	and	a4,a4,1
42007e86:	00137693          	and	a3,t1,1
42007e8a:	3c075637          	lui	a2,0x3c075
42007e8e:	92c60613          	add	a2,a2,-1748 # 3c07492c <_flash_rodata_start+0x480c>
42007e92:	5dc530ef          	jal	4205b46e <snprintf>
42007e96:	bd6d                	j	42007d50 <describe_loco_instr+0x82>
# 	} else if (b == 0xDE && n >= 2) {
42007e98:	4605                	li	a2,1
42007e9a:	e8f652e3          	bge	a2,a5,42007d1e <describe_loco_instr+0x50>
# 		snprintf(buf, size, "F13-F20=0x%02X", p[1]);
42007e9e:	00174683          	lbu	a3,1(a4)
42007ea2:	3c075637          	lui	a2,0x3c075
42007ea6:	93c60613          	add	a2,a2,-1732 # 3c07493c <_flash_rodata_start+0x481c>
42007eaa:	5c4530ef          	jal	4205b46e <snprintf>
42007eae:	b54d                	j	42007d50 <describe_loco_instr+0x82>
# 	} else if (b == 0xDF && n >= 2) {
42007eb0:	4605                	li	a2,1
42007eb2:	e6f65ae3          	bge	a2,a5,42007d26 <describe_loco_instr+0x58>
# 		snprintf(buf, size, "F21-F28=0x%02X", p[1]);
42007eb6:	00174683          	lbu	a3,1(a4)
42007eba:	3c075637          	lui	a2,0x3c075
42007ebe:	94c60613          	add	a2,a2,-1716 # 3c07494c <_flash_rodata_start+0x482c>
42007ec2:	5ac530ef          	jal	4205b46e <snprintf>
42007ec6:	b569                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "CV access (POM)");
42007ec8:	3c075637          	lui	a2,0x3c075
42007ecc:	95c60613          	add	a2,a2,-1700 # 3c07495c <_flash_rodata_start+0x483c>
42007ed0:	59e530ef          	jal	4205b46e <snprintf>
42007ed4:	bdb5                	j	42007d50 <describe_loco_instr+0x82>
# 		snprintf(buf, size, "instruction 0x%02X", b);
42007ed6:	869a                	mv	a3,t1
42007ed8:	3c075637          	lui	a2,0x3c075
42007edc:	98c60613          	add	a2,a2,-1652 # 3c07498c <_flash_rodata_start+0x486c>
42007ee0:	58e530ef          	jal	4205b46e <snprintf>
42007ee4:	b5b5                	j	42007d50 <describe_loco_instr+0x82>

42007ee6 <cmd_dcc_stats>:
# 	       printed, stats.packets_ok - ok_before);
# 	return 0;
# }
# 
# static int cmd_dcc_stats(bool clear)
# {
42007ee6:	1141                	add	sp,sp,-16
42007ee8:	c606                	sw	ra,12(sp)
42007eea:	c422                	sw	s0,8(sp)
# 	if (clear) {
42007eec:	e949                	bnez	a0,42007f7e <cmd_dcc_stats+0x98>
# 		memset((void *)&stats, 0, sizeof(stats));
# 		printf("cleared\n");
# 		return 0;
# 	}
# 
# 	printf("edges       %" PRIu32 "\n", stats.edges);
42007eee:	3fc99437          	lui	s0,0x3fc99
42007ef2:	9a440413          	add	s0,s0,-1628 # 3fc989a4 <stats>
42007ef6:	400c                	lw	a1,0(s0)
42007ef8:	3c075537          	lui	a0,0x3c075
42007efc:	9a850513          	add	a0,a0,-1624 # 3c0749a8 <_flash_rodata_start+0x4888>
42007f00:	002530ef          	jal	4205af02 <printf>
# 	printf("packets_ok  %" PRIu32 "\n", stats.packets_ok);
42007f04:	404c                	lw	a1,4(s0)
42007f06:	3c075537          	lui	a0,0x3c075
42007f0a:	9bc50513          	add	a0,a0,-1604 # 3c0749bc <_flash_rodata_start+0x489c>
42007f0e:	7f5520ef          	jal	4205af02 <printf>
# 	printf("bad_timing  %" PRIu32 "\n", stats.bad_timing);
42007f12:	440c                	lw	a1,8(s0)
42007f14:	3c075537          	lui	a0,0x3c075
42007f18:	9d050513          	add	a0,a0,-1584 # 3c0749d0 <_flash_rodata_start+0x48b0>
42007f1c:	7e7520ef          	jal	4205af02 <printf>
# 	printf("bad_pair    %" PRIu32 "\n", stats.bad_pair);
42007f20:	444c                	lw	a1,12(s0)
42007f22:	3c075537          	lui	a0,0x3c075
42007f26:	9e450513          	add	a0,a0,-1564 # 3c0749e4 <_flash_rodata_start+0x48c4>
42007f2a:	7d9520ef          	jal	4205af02 <printf>
# 	printf("cutouts     %" PRIu32 " (RailCom, not an error)\n", stats.cutouts);
42007f2e:	480c                	lw	a1,16(s0)
42007f30:	3c075537          	lui	a0,0x3c075
42007f34:	9f850513          	add	a0,a0,-1544 # 3c0749f8 <_flash_rodata_start+0x48d8>
42007f38:	7cb520ef          	jal	4205af02 <printf>
# 	printf("bad_xor     %" PRIu32 "\n", stats.bad_xor);
42007f3c:	484c                	lw	a1,20(s0)
42007f3e:	3c075537          	lui	a0,0x3c075
42007f42:	a2450513          	add	a0,a0,-1500 # 3c074a24 <_flash_rodata_start+0x4904>
42007f46:	7bd520ef          	jal	4205af02 <printf>
# 	printf("too_short   %" PRIu32 "\n", stats.too_short);
42007f4a:	4c0c                	lw	a1,24(s0)
42007f4c:	3c075537          	lui	a0,0x3c075
42007f50:	a3850513          	add	a0,a0,-1480 # 3c074a38 <_flash_rodata_start+0x4918>
42007f54:	7af520ef          	jal	4205af02 <printf>
# 	printf("overflow    %" PRIu32 "\n", stats.overflow);
42007f58:	4c4c                	lw	a1,28(s0)
42007f5a:	3c075537          	lui	a0,0x3c075
42007f5e:	a4c50513          	add	a0,a0,-1460 # 3c074a4c <_flash_rodata_start+0x492c>
42007f62:	7a1520ef          	jal	4205af02 <printf>
# 	printf("queue_full  %" PRIu32 " (normal while nothing is sniffing)\n", stats.queue_full);
42007f66:	500c                	lw	a1,32(s0)
42007f68:	3c075537          	lui	a0,0x3c075
42007f6c:	a6050513          	add	a0,a0,-1440 # 3c074a60 <_flash_rodata_start+0x4940>
42007f70:	793520ef          	jal	4205af02 <printf>
# 	return 0;
# }
42007f74:	4501                	li	a0,0
42007f76:	40b2                	lw	ra,12(sp)
42007f78:	4422                	lw	s0,8(sp)
42007f7a:	0141                	add	sp,sp,16
42007f7c:	8082                	ret
# 		memset((void *)&stats, 0, sizeof(stats));
42007f7e:	3fc997b7          	lui	a5,0x3fc99
42007f82:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
42007f86:	0007a023          	sw	zero,0(a5)
42007f8a:	0007a223          	sw	zero,4(a5)
42007f8e:	0007a423          	sw	zero,8(a5)
42007f92:	0007a623          	sw	zero,12(a5)
42007f96:	0007a823          	sw	zero,16(a5)
42007f9a:	0007aa23          	sw	zero,20(a5)
42007f9e:	0007ac23          	sw	zero,24(a5)
42007fa2:	0007ae23          	sw	zero,28(a5)
42007fa6:	0207a023          	sw	zero,32(a5)
# 		printf("cleared\n");
42007faa:	3c075537          	lui	a0,0x3c075
42007fae:	9a050513          	add	a0,a0,-1632 # 3c0749a0 <_flash_rodata_start+0x4880>
42007fb2:	0e4530ef          	jal	4205b096 <puts>
# 		return 0;
42007fb6:	bf7d                	j	42007f74 <cmd_dcc_stats+0x8e>

42007fb8 <seen_recently>:
# {
42007fb8:	1141                	add	sp,sp,-16
42007fba:	c606                	sw	ra,12(sp)
42007fbc:	c422                	sw	s0,8(sp)
42007fbe:	c226                	sw	s1,4(sp)
42007fc0:	84aa                	mv	s1,a0
# 	for (int i = 0; i < RECENT_LEN; i++) {
42007fc2:	4401                	li	s0,0
42007fc4:	a811                	j	42007fd8 <seen_recently+0x20>
# 			bool repeat = p->time_us - r->time_us < REPEAT_WINDOW_US;
42007fc6:	e3c9                	bnez	a5,42008048 <seen_recently+0x90>
42007fc8:	000f47b7          	lui	a5,0xf4
42007fcc:	23f78793          	add	a5,a5,575 # f423f <_rtc_fast_length+0xf403f>
42007fd0:	06e7fc63          	bgeu	a5,a4,42008048 <seen_recently+0x90>
42007fd4:	a88d                	j	42008046 <seen_recently+0x8e>
# 	for (int i = 0; i < RECENT_LEN; i++) {
42007fd6:	0405                	add	s0,s0,1
42007fd8:	47bd                	li	a5,15
42007fda:	0887c563          	blt	a5,s0,42008064 <seen_recently+0xac>
# 		if (r->len == p->len && memcmp(r->data, p->data, p->len) == 0) {
42007fde:	3fc987b7          	lui	a5,0x3fc98
42007fe2:	00141713          	sll	a4,s0,0x1
42007fe6:	9722                	add	a4,a4,s0
42007fe8:	070e                	sll	a4,a4,0x3
42007fea:	7b878793          	add	a5,a5,1976 # 3fc987b8 <recent>
42007fee:	97ba                	add	a5,a5,a4
42007ff0:	0087c783          	lbu	a5,8(a5)
42007ff4:	0084c603          	lbu	a2,8(s1)
42007ff8:	fcc79fe3          	bne	a5,a2,42007fd6 <seen_recently+0x1e>
42007ffc:	3fc987b7          	lui	a5,0x3fc98
42008000:	7b878793          	add	a5,a5,1976 # 3fc987b8 <recent>
42008004:	00f70533          	add	a0,a4,a5
42008008:	00948593          	add	a1,s1,9
4200800c:	0525                	add	a0,a0,9
4200800e:	fdff8097          	auipc	ra,0xfdff8
42008012:	352080e7          	jalr	850(ra) # 40000360 <memcmp>
42008016:	f161                	bnez	a0,42007fd6 <seen_recently+0x1e>
# 			bool repeat = p->time_us - r->time_us < REPEAT_WINDOW_US;
42008018:	4094                	lw	a3,0(s1)
4200801a:	40d0                	lw	a2,4(s1)
4200801c:	3fc987b7          	lui	a5,0x3fc98
42008020:	00141713          	sll	a4,s0,0x1
42008024:	9722                	add	a4,a4,s0
42008026:	070e                	sll	a4,a4,0x3
42008028:	7b878793          	add	a5,a5,1976 # 3fc987b8 <recent>
4200802c:	97ba                	add	a5,a5,a4
4200802e:	4398                	lw	a4,0(a5)
42008030:	43dc                	lw	a5,4(a5)
42008032:	40e68733          	sub	a4,a3,a4
42008036:	00e6b5b3          	sltu	a1,a3,a4
4200803a:	40f607b3          	sub	a5,a2,a5
4200803e:	8f8d                	sub	a5,a5,a1
42008040:	4505                	li	a0,1
42008042:	f8f052e3          	blez	a5,42007fc6 <seen_recently+0xe>
42008046:	4501                	li	a0,0
42008048:	0ff57513          	zext.b	a0,a0
# 			r->time_us = p->time_us;
4200804c:	3fc987b7          	lui	a5,0x3fc98
42008050:	00141713          	sll	a4,s0,0x1
42008054:	9722                	add	a4,a4,s0
42008056:	070e                	sll	a4,a4,0x3
42008058:	7b878793          	add	a5,a5,1976 # 3fc987b8 <recent>
4200805c:	97ba                	add	a5,a5,a4
4200805e:	c394                	sw	a3,0(a5)
42008060:	c3d0                	sw	a2,4(a5)
# 			return repeat;
42008062:	a891                	j	420080b6 <seen_recently+0xfe>
# 	recent[recent_next] = *p;
42008064:	3fc99637          	lui	a2,0x3fc99
42008068:	34862783          	lw	a5,840(a2) # 3fc99348 <recent_next>
4200806c:	3fc98737          	lui	a4,0x3fc98
42008070:	00179693          	sll	a3,a5,0x1
42008074:	96be                	add	a3,a3,a5
42008076:	068e                	sll	a3,a3,0x3
42008078:	7b870713          	add	a4,a4,1976 # 3fc987b8 <recent>
4200807c:	9736                	add	a4,a4,a3
4200807e:	0004a303          	lw	t1,0(s1)
42008082:	0044a883          	lw	a7,4(s1)
42008086:	0084a803          	lw	a6,8(s1)
4200808a:	44c8                	lw	a0,12(s1)
4200808c:	488c                	lw	a1,16(s1)
4200808e:	48d4                	lw	a3,20(s1)
42008090:	00672023          	sw	t1,0(a4)
42008094:	01172223          	sw	a7,4(a4)
42008098:	01072423          	sw	a6,8(a4)
4200809c:	c748                	sw	a0,12(a4)
4200809e:	cb0c                	sw	a1,16(a4)
420080a0:	cb54                	sw	a3,20(a4)
# 	recent_next = (recent_next + 1) % RECENT_LEN;
420080a2:	0785                	add	a5,a5,1
420080a4:	41f7d713          	sra	a4,a5,0x1f
420080a8:	8371                	srl	a4,a4,0x1c
420080aa:	97ba                	add	a5,a5,a4
420080ac:	8bbd                	and	a5,a5,15
420080ae:	8f99                	sub	a5,a5,a4
420080b0:	34f62423          	sw	a5,840(a2)
# 	return false;
420080b4:	4501                	li	a0,0
# }
420080b6:	40b2                	lw	ra,12(sp)
420080b8:	4422                	lw	s0,8(sp)
420080ba:	4492                	lw	s1,4(sp)
420080bc:	0141                	add	sp,sp,16
420080be:	8082                	ret

420080c0 <cmd_dcc_timing>:
# 
# /* Hardware bring-up aid: histogram of edge-to-edge times. A healthy
#  * opto stage shows one peak at 50-60 us ("1" halves) and one at
#  * 90-120 us ("0" halves), and nothing else to speak of. */
# static int cmd_dcc_timing(int ms)
# {
420080c0:	7135                	add	sp,sp,-160
420080c2:	cf06                	sw	ra,156(sp)
420080c4:	cd22                	sw	s0,152(sp)
420080c6:	cb26                	sw	s1,148(sp)
420080c8:	c94a                	sw	s2,144(sp)
420080ca:	c74e                	sw	s3,140(sp)
420080cc:	842a                	mv	s0,a0
# 	printf("Collecting half-bit timings for %dms...\n", ms);
420080ce:	85aa                	mv	a1,a0
420080d0:	3c075537          	lui	a0,0x3c075
420080d4:	a9450513          	add	a0,a0,-1388 # 3c074a94 <_flash_rodata_start+0x4974>
420080d8:	62b520ef          	jal	4205af02 <printf>
# 	memset((void *)hist, 0, sizeof(hist));
420080dc:	05400613          	li	a2,84
420080e0:	4581                	li	a1,0
420080e2:	3fc99537          	lui	a0,0x3fc99
420080e6:	95050513          	add	a0,a0,-1712 # 3fc98950 <hist>
420080ea:	fdff8097          	auipc	ra,0xfdff8
420080ee:	26a080e7          	jalr	618(ra) # 40000354 <memset>
# 	vTaskDelay(pdMS_TO_TICKS(ms));
420080f2:	06400793          	li	a5,100
420080f6:	02f40433          	mul	s0,s0,a5
420080fa:	3e800513          	li	a0,1000
420080fe:	02a45533          	divu	a0,s0,a0
42008102:	fe387097          	auipc	ra,0xfe387
42008106:	adc080e7          	jalr	-1316(ra) # 4038ebde <vTaskDelay>
# 
# 	uint32_t snap[HIST_BUCKETS];
# 	uint32_t max = 0;
# 	for (int i = 0; i < HIST_BUCKETS; i++) {
4200810a:	4681                	li	a3,0
# 	uint32_t max = 0;
4200810c:	4481                	li	s1,0
# 	for (int i = 0; i < HIST_BUCKETS; i++) {
4200810e:	a011                	j	42008112 <cmd_dcc_timing+0x52>
42008110:	0685                	add	a3,a3,1
42008112:	47d1                	li	a5,20
42008114:	02d7c363          	blt	a5,a3,4200813a <cmd_dcc_timing+0x7a>
# 		snap[i] = hist[i];
42008118:	3fc99737          	lui	a4,0x3fc99
4200811c:	00269793          	sll	a5,a3,0x2
42008120:	95070713          	add	a4,a4,-1712 # 3fc98950 <hist>
42008124:	973e                	add	a4,a4,a5
42008126:	4318                	lw	a4,0(a4)
42008128:	08078793          	add	a5,a5,128
4200812c:	978a                	add	a5,a5,sp
4200812e:	fae7a623          	sw	a4,-84(a5)
# 		if (snap[i] > max) {
42008132:	fce4ffe3          	bgeu	s1,a4,42008110 <cmd_dcc_timing+0x50>
# 			max = snap[i];
42008136:	84ba                	mv	s1,a4
42008138:	bfe1                	j	42008110 <cmd_dcc_timing+0x50>
# 		}
# 	}
# 
# 	if (max == 0) {
4200813a:	c099                	beqz	s1,42008140 <cmd_dcc_timing+0x80>
# 		printf("No edges on GPIO%d -- check the opto wiring and track power.\n",
# 		       DCC_IN_GPIO);
# 		return 1;
# 	}
# 
# 	for (int i = 0; i < HIST_BUCKETS; i++) {
4200813c:	4401                	li	s0,0
4200813e:	a03d                	j	4200816c <cmd_dcc_timing+0xac>
# 		printf("No edges on GPIO%d -- check the opto wiring and track power.\n",
42008140:	4591                	li	a1,4
42008142:	3c075537          	lui	a0,0x3c075
42008146:	ac050513          	add	a0,a0,-1344 # 3c074ac0 <_flash_rodata_start+0x49a0>
4200814a:	5b9520ef          	jal	4205af02 <printf>
# 		return 1;
4200814e:	4505                	li	a0,1
42008150:	a06d                	j	420081fa <cmd_dcc_timing+0x13a>
# 		char bar[41];
# 		int len = (int)((uint64_t)snap[i] * 40 / max);
# 		memset(bar, '#', len);
# 		bar[len] = '\0';
# 		if (i == HIST_BUCKETS - 1) {
# 			printf("   >=%3d us %7" PRIu32 " %s\n", i * HIST_BUCKET_US, snap[i], bar);
42008152:	00241593          	sll	a1,s0,0x2
42008156:	95a2                	add	a1,a1,s0
42008158:	868a                	mv	a3,sp
4200815a:	864a                	mv	a2,s2
4200815c:	0586                	sll	a1,a1,0x1
4200815e:	3c075537          	lui	a0,0x3c075
42008162:	b0050513          	add	a0,a0,-1280 # 3c074b00 <_flash_rodata_start+0x49e0>
42008166:	59d520ef          	jal	4205af02 <printf>
# 	for (int i = 0; i < HIST_BUCKETS; i++) {
4200816a:	0405                	add	s0,s0,1
4200816c:	47d1                	li	a5,20
4200816e:	0887c563          	blt	a5,s0,420081f8 <cmd_dcc_timing+0x138>
# 		if (snap[i] == 0) {
42008172:	00241793          	sll	a5,s0,0x2
42008176:	08078793          	add	a5,a5,128
4200817a:	978a                	add	a5,a5,sp
4200817c:	fac7a903          	lw	s2,-84(a5)
42008180:	fe0905e3          	beqz	s2,4200816a <cmd_dcc_timing+0xaa>
# 		int len = (int)((uint64_t)snap[i] * 40 / max);
42008184:	01e95793          	srl	a5,s2,0x1e
42008188:	00291593          	sll	a1,s2,0x2
4200818c:	01258533          	add	a0,a1,s2
42008190:	00b535b3          	sltu	a1,a0,a1
42008194:	95be                	add	a1,a1,a5
42008196:	01d55793          	srl	a5,a0,0x1d
4200819a:	058e                	sll	a1,a1,0x3
4200819c:	8626                	mv	a2,s1
4200819e:	4681                	li	a3,0
420081a0:	050e                	sll	a0,a0,0x3
420081a2:	8ddd                	or	a1,a1,a5
420081a4:	fdff8097          	auipc	ra,0xfdff8
420081a8:	708080e7          	jalr	1800(ra) # 400008ac <__udivdi3>
420081ac:	89aa                	mv	s3,a0
# 		memset(bar, '#', len);
420081ae:	862a                	mv	a2,a0
420081b0:	02300593          	li	a1,35
420081b4:	850a                	mv	a0,sp
420081b6:	fdff8097          	auipc	ra,0xfdff8
420081ba:	19e080e7          	jalr	414(ra) # 40000354 <memset>
# 		bar[len] = '\0';
420081be:	08098793          	add	a5,s3,128
420081c2:	002789b3          	add	s3,a5,sp
420081c6:	f8098023          	sb	zero,-128(s3)
# 		if (i == HIST_BUCKETS - 1) {
420081ca:	47d1                	li	a5,20
420081cc:	f8f403e3          	beq	s0,a5,42008152 <cmd_dcc_timing+0x92>
# 		} else {
# 			printf("%3d-%3d us %7" PRIu32 " %s\n", i * HIST_BUCKET_US,
# 			       i * HIST_BUCKET_US + HIST_BUCKET_US - 1, snap[i], bar);
420081d0:	00140793          	add	a5,s0,1
420081d4:	00279613          	sll	a2,a5,0x2
420081d8:	963e                	add	a2,a2,a5
420081da:	0606                	sll	a2,a2,0x1
# 			printf("%3d-%3d us %7" PRIu32 " %s\n", i * HIST_BUCKET_US,
420081dc:	00241593          	sll	a1,s0,0x2
420081e0:	95a2                	add	a1,a1,s0
420081e2:	870a                	mv	a4,sp
420081e4:	86ca                	mv	a3,s2
420081e6:	167d                	add	a2,a2,-1
420081e8:	0586                	sll	a1,a1,0x1
420081ea:	3c075537          	lui	a0,0x3c075
420081ee:	b1850513          	add	a0,a0,-1256 # 3c074b18 <_flash_rodata_start+0x49f8>
420081f2:	511520ef          	jal	4205af02 <printf>
420081f6:	bf95                	j	4200816a <cmd_dcc_timing+0xaa>
# 		}
# 	}
# 	return 0;
420081f8:	4501                	li	a0,0
# }
420081fa:	40fa                	lw	ra,156(sp)
420081fc:	446a                	lw	s0,152(sp)
420081fe:	44da                	lw	s1,148(sp)
42008200:	494a                	lw	s2,144(sp)
42008202:	49ba                	lw	s3,140(sp)
42008204:	610d                	add	sp,sp,160
42008206:	8082                	ret

42008208 <dcc_describe>:
# {
42008208:	715d                	add	sp,sp,-80
4200820a:	c686                	sw	ra,76(sp)
4200820c:	c4a2                	sw	s0,72(sp)
4200820e:	c2a6                	sw	s1,68(sp)
42008210:	c0ca                	sw	s2,64(sp)
42008212:	de4e                	sw	s3,60(sp)
42008214:	842a                	mv	s0,a0
42008216:	89ae                	mv	s3,a1
42008218:	8932                	mv	s2,a2
# 	const uint8_t *d = p->data;
4200821a:	0525                	add	a0,a0,9
# 	int n = p->len - 1;  /* without the checksum */
4200821c:	00844583          	lbu	a1,8(s0)
42008220:	fff58713          	add	a4,a1,-1
# 	if (n == 2 && d[0] == 0xFF && d[1] == 0x00) {
42008224:	4789                	li	a5,2
42008226:	04f70363          	beq	a4,a5,4200826c <dcc_describe+0x64>
# 	} else if (d[0] == 0x00) {
4200822a:	00944783          	lbu	a5,9(s0)
4200822e:	ebb5                	bnez	a5,420082a2 <dcc_describe+0x9a>
# 		if (n == 2 && d[1] == 0x00) {
42008230:	4789                	li	a5,2
42008232:	00f71563          	bne	a4,a5,4200823c <dcc_describe+0x34>
42008236:	00a44783          	lbu	a5,10(s0)
4200823a:	cbb9                	beqz	a5,42008290 <dcc_describe+0x88>
# 			describe_loco_instr(d + 1, n - 1, instr, sizeof(instr));
4200823c:	03000693          	li	a3,48
42008240:	860a                	mv	a2,sp
42008242:	15f9                	add	a1,a1,-2
42008244:	00a40513          	add	a0,s0,10
42008248:	a87ff0ef          	jal	42007cce <describe_loco_instr>
# 			snprintf(buf, size, "BROADCAST %s", instr);
4200824c:	868a                	mv	a3,sp
4200824e:	3c075637          	lui	a2,0x3c075
42008252:	b3460613          	add	a2,a2,-1228 # 3c074b34 <_flash_rodata_start+0x4a14>
42008256:	85ca                	mv	a1,s2
42008258:	854e                	mv	a0,s3
4200825a:	214530ef          	jal	4205b46e <snprintf>
# }
4200825e:	40b6                	lw	ra,76(sp)
42008260:	4426                	lw	s0,72(sp)
42008262:	4496                	lw	s1,68(sp)
42008264:	4906                	lw	s2,64(sp)
42008266:	59f2                	lw	s3,60(sp)
42008268:	6161                	add	sp,sp,80
4200826a:	8082                	ret
# 	if (n == 2 && d[0] == 0xFF && d[1] == 0x00) {
4200826c:	00944683          	lbu	a3,9(s0)
42008270:	0ff00793          	li	a5,255
42008274:	faf69be3          	bne	a3,a5,4200822a <dcc_describe+0x22>
42008278:	00a44783          	lbu	a5,10(s0)
4200827c:	f7dd                	bnez	a5,4200822a <dcc_describe+0x22>
# 		snprintf(buf, size, "IDLE");
4200827e:	3c074637          	lui	a2,0x3c074
42008282:	cd860613          	add	a2,a2,-808 # 3c073cd8 <_flash_rodata_start+0x3bb8>
42008286:	85ca                	mv	a1,s2
42008288:	854e                	mv	a0,s3
4200828a:	1e4530ef          	jal	4205b46e <snprintf>
4200828e:	bfc1                	j	4200825e <dcc_describe+0x56>
# 			snprintf(buf, size, "RESET");
42008290:	3c075637          	lui	a2,0x3c075
42008294:	b2c60613          	add	a2,a2,-1236 # 3c074b2c <_flash_rodata_start+0x4a0c>
42008298:	85ca                	mv	a1,s2
4200829a:	854e                	mv	a0,s3
4200829c:	1d2530ef          	jal	4205b46e <snprintf>
420082a0:	bf7d                	j	4200825e <dcc_describe+0x56>
# 	} else if (d[0] <= 0x7F) {
420082a2:	01879693          	sll	a3,a5,0x18
420082a6:	86e1                	sra	a3,a3,0x18
420082a8:	0206d663          	bgez	a3,420082d4 <dcc_describe+0xcc>
# 	} else if (d[0] <= 0xBF) {
420082ac:	0bf00693          	li	a3,191
420082b0:	04f6f663          	bgeu	a3,a5,420082fc <dcc_describe+0xf4>
# 	} else if (d[0] <= 0xE7 && n >= 2) {
420082b4:	0e700693          	li	a3,231
420082b8:	00f6e563          	bltu	a3,a5,420082c2 <dcc_describe+0xba>
420082bc:	4685                	li	a3,1
420082be:	04e6c563          	blt	a3,a4,42008308 <dcc_describe+0x100>
# 		snprintf(buf, size, "OTHER");
420082c2:	3c075637          	lui	a2,0x3c075
420082c6:	b6460613          	add	a2,a2,-1180 # 3c074b64 <_flash_rodata_start+0x4a44>
420082ca:	85ca                	mv	a1,s2
420082cc:	854e                	mv	a0,s3
420082ce:	1a0530ef          	jal	4205b46e <snprintf>
# }
420082d2:	b771                	j	4200825e <dcc_describe+0x56>
# 		describe_loco_instr(d + 1, n - 1, instr, sizeof(instr));
420082d4:	03000693          	li	a3,48
420082d8:	860a                	mv	a2,sp
420082da:	15f9                	add	a1,a1,-2
420082dc:	00a40513          	add	a0,s0,10
420082e0:	9efff0ef          	jal	42007cce <describe_loco_instr>
# 		snprintf(buf, size, "LOCO %u %s", d[0], instr);
420082e4:	870a                	mv	a4,sp
420082e6:	00944683          	lbu	a3,9(s0)
420082ea:	3c075637          	lui	a2,0x3c075
420082ee:	b4460613          	add	a2,a2,-1212 # 3c074b44 <_flash_rodata_start+0x4a24>
420082f2:	85ca                	mv	a1,s2
420082f4:	854e                	mv	a0,s3
420082f6:	178530ef          	jal	4205b46e <snprintf>
420082fa:	b795                	j	4200825e <dcc_describe+0x56>
# 		describe_accessory(d, n, buf, size);
420082fc:	86ca                	mv	a3,s2
420082fe:	864e                	mv	a2,s3
42008300:	85ba                	mv	a1,a4
42008302:	8c7ff0ef          	jal	42007bc8 <describe_accessory>
42008306:	bfa1                	j	4200825e <dcc_describe+0x56>
# 		unsigned addr = ((unsigned)(d[0] & 0x3F) << 8) | d[1];
42008308:	00879493          	sll	s1,a5,0x8
4200830c:	6791                	lui	a5,0x4
4200830e:	f0078793          	add	a5,a5,-256 # 3f00 <_rtc_fast_length+0x3d00>
42008312:	8cfd                	and	s1,s1,a5
42008314:	00a44783          	lbu	a5,10(s0)
42008318:	8cdd                	or	s1,s1,a5
# 		describe_loco_instr(d + 2, n - 2, instr, sizeof(instr));
4200831a:	03000693          	li	a3,48
4200831e:	860a                	mv	a2,sp
42008320:	15f5                	add	a1,a1,-3
42008322:	00b40513          	add	a0,s0,11
42008326:	9a9ff0ef          	jal	42007cce <describe_loco_instr>
# 		snprintf(buf, size, "LOCO %u (long) %s", addr, instr);
4200832a:	870a                	mv	a4,sp
4200832c:	86a6                	mv	a3,s1
4200832e:	3c075637          	lui	a2,0x3c075
42008332:	b5060613          	add	a2,a2,-1200 # 3c074b50 <_flash_rodata_start+0x4a30>
42008336:	85ca                	mv	a1,s2
42008338:	854e                	mv	a0,s3
4200833a:	134530ef          	jal	4205b46e <snprintf>
# 	} else if (d[0] <= 0xE7 && n >= 2) {
4200833e:	b705                	j	4200825e <dcc_describe+0x56>

42008340 <dcc_receive>:
# {
42008340:	1141                	add	sp,sp,-16
42008342:	c606                	sw	ra,12(sp)
42008344:	862e                	mv	a2,a1
# 	return xQueueReceive(dcc_rx_queue, p, timeout) == pdTRUE;
42008346:	85aa                	mv	a1,a0
42008348:	3fc997b7          	lui	a5,0x3fc99
4200834c:	3687a503          	lw	a0,872(a5) # 3fc99368 <dcc_rx_queue>
42008350:	fe385097          	auipc	ra,0xfe385
42008354:	1c2080e7          	jalr	450(ra) # 4038d512 <xQueueReceive>
42008358:	157d                	add	a0,a0,-1
# }
4200835a:	00153513          	seqz	a0,a0
4200835e:	40b2                	lw	ra,12(sp)
42008360:	0141                	add	sp,sp,16
42008362:	8082                	ret

42008364 <cmd_dcc_sniff>:
# {
42008364:	7171                	add	sp,sp,-176
42008366:	d706                	sw	ra,172(sp)
42008368:	d522                	sw	s0,168(sp)
4200836a:	d326                	sw	s1,164(sp)
4200836c:	d14a                	sw	s2,160(sp)
4200836e:	cf4e                	sw	s3,156(sp)
42008370:	cd52                	sw	s4,152(sp)
42008372:	842a                	mv	s0,a0
42008374:	892e                	mv	s2,a1
# 	printf("Sniffing for %ds%s...\n", seconds,
42008376:	c9bd                	beqz	a1,420083ec <cmd_dcc_sniff+0x88>
42008378:	3c075637          	lui	a2,0x3c075
4200837c:	b6c60613          	add	a2,a2,-1172 # 3c074b6c <_flash_rodata_start+0x4a4c>
42008380:	85a2                	mv	a1,s0
42008382:	3c075537          	lui	a0,0x3c075
42008386:	ba850513          	add	a0,a0,-1112 # 3c074ba8 <_flash_rodata_start+0x4a88>
4200838a:	379520ef          	jal	4205af02 <printf>
# 	memset(recent, 0, sizeof(recent));
4200838e:	18000613          	li	a2,384
42008392:	4581                	li	a1,0
42008394:	3fc98537          	lui	a0,0x3fc98
42008398:	7b850513          	add	a0,a0,1976 # 3fc987b8 <recent>
4200839c:	fdff8097          	auipc	ra,0xfdff8
420083a0:	fb8080e7          	jalr	-72(ra) # 40000354 <memset>
# 	recent_next = 0;
420083a4:	3fc997b7          	lui	a5,0x3fc99
420083a8:	3407a423          	sw	zero,840(a5) # 3fc99348 <recent_next>
# 	xQueueReset(dcc_rx_queue);
420083ac:	4581                	li	a1,0
420083ae:	3fc997b7          	lui	a5,0x3fc99
420083b2:	3687a503          	lw	a0,872(a5) # 3fc99368 <dcc_rx_queue>
420083b6:	fe385097          	auipc	ra,0xfe385
420083ba:	8f4080e7          	jalr	-1804(ra) # 4038ccaa <xQueueGenericReset>
# 	uint32_t ok_before = stats.packets_ok;
420083be:	3fc997b7          	lui	a5,0x3fc99
420083c2:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
420083c6:	0047aa03          	lw	s4,4(a5)
# 	TickType_t end = xTaskGetTickCount() + pdMS_TO_TICKS(seconds * 1000);
420083ca:	fe386097          	auipc	ra,0xfe386
420083ce:	466080e7          	jalr	1126(ra) # 4038e830 <xTaskGetTickCount>
420083d2:	67e1                	lui	a5,0x18
420083d4:	6a078793          	add	a5,a5,1696 # 186a0 <_rtc_fast_length+0x184a0>
420083d8:	02f40433          	mul	s0,s0,a5
420083dc:	3e800793          	li	a5,1000
420083e0:	02f45433          	divu	s0,s0,a5
420083e4:	008504b3          	add	s1,a0,s0
# 	unsigned printed = 0;
420083e8:	4981                	li	s3,0
# 	while (xTaskGetTickCount() < end) {
420083ea:	a8b5                	j	42008466 <cmd_dcc_sniff+0x102>
# 	printf("Sniffing for %ds%s...\n", seconds,
420083ec:	3c075637          	lui	a2,0x3c075
420083f0:	b7c60613          	add	a2,a2,-1156 # 3c074b7c <_flash_rodata_start+0x4a5c>
420083f4:	b771                	j	42008380 <cmd_dcc_sniff+0x1c>
# 			snprintf(hex + i * 3, sizeof(hex) - i * 3, "%02X ", p.data[i]);
420083f6:	00141513          	sll	a0,s0,0x1
420083fa:	9522                	add	a0,a0,s0
420083fc:	09040793          	add	a5,s0,144
42008400:	978a                	add	a5,a5,sp
42008402:	f817c683          	lbu	a3,-127(a5)
42008406:	3c075637          	lui	a2,0x3c075
4200840a:	bc060613          	add	a2,a2,-1088 # 3c074bc0 <_flash_rodata_start+0x4aa0>
4200840e:	45e5                	li	a1,25
42008410:	8d89                	sub	a1,a1,a0
42008412:	105c                	add	a5,sp,36
42008414:	953e                	add	a0,a0,a5
42008416:	058530ef          	jal	4205b46e <snprintf>
# 		for (int i = 0; i < p.len; i++) {
4200841a:	0405                	add	s0,s0,1
4200841c:	01014783          	lbu	a5,16(sp)
42008420:	fcf44be3          	blt	s0,a5,420083f6 <cmd_dcc_sniff+0x92>
# 		dcc_describe(&p, desc, sizeof(desc));
42008424:	05000613          	li	a2,80
42008428:	008c                	add	a1,sp,64
4200842a:	0028                	add	a0,sp,8
4200842c:	dddff0ef          	jal	42008208 <dcc_describe>
# 		printf("%10.3f  %-18s %s\n", p.time_us / 1e6, hex, desc);
42008430:	4522                	lw	a0,8(sp)
42008432:	45b2                	lw	a1,12(sp)
42008434:	fdff8097          	auipc	ra,0xfdff8
42008438:	3c0080e7          	jalr	960(ra) # 400007f4 <__floatdidf>
4200843c:	3c07e7b7          	lui	a5,0x3c07e
42008440:	a607a603          	lw	a2,-1440(a5) # 3c07da60 <__func__.1+0x1c>
42008444:	a647a683          	lw	a3,-1436(a5)
42008448:	fdff8097          	auipc	ra,0xfdff8
4200844c:	368080e7          	jalr	872(ra) # 400007b0 <__divdf3>
42008450:	862a                	mv	a2,a0
42008452:	86ae                	mv	a3,a1
42008454:	009c                	add	a5,sp,64
42008456:	1058                	add	a4,sp,36
42008458:	3c075537          	lui	a0,0x3c075
4200845c:	bc850513          	add	a0,a0,-1080 # 3c074bc8 <_flash_rodata_start+0x4aa8>
42008460:	2a3520ef          	jal	4205af02 <printf>
# 		printed++;
42008464:	0985                	add	s3,s3,1
# 	while (xTaskGetTickCount() < end) {
42008466:	fe386097          	auipc	ra,0xfe386
4200846a:	3ca080e7          	jalr	970(ra) # 4038e830 <xTaskGetTickCount>
4200846e:	02957b63          	bgeu	a0,s1,420084a4 <cmd_dcc_sniff+0x140>
# 		if (!dcc_receive(&p, pdMS_TO_TICKS(200))) {
42008472:	45d1                	li	a1,20
42008474:	0028                	add	a0,sp,8
42008476:	ecbff0ef          	jal	42008340 <dcc_receive>
4200847a:	d575                	beqz	a0,42008466 <cmd_dcc_sniff+0x102>
# 		if (!all && (is_idle(&p) || seen_recently(&p))) {
4200847c:	00091a63          	bnez	s2,42008490 <cmd_dcc_sniff+0x12c>
42008480:	0028                	add	a0,sp,8
42008482:	f26ff0ef          	jal	42007ba8 <is_idle>
42008486:	f165                	bnez	a0,42008466 <cmd_dcc_sniff+0x102>
42008488:	0028                	add	a0,sp,8
4200848a:	b2fff0ef          	jal	42007fb8 <seen_recently>
4200848e:	fd61                	bnez	a0,42008466 <cmd_dcc_sniff+0x102>
# 		char hex[DCC_MAX_BYTES * 3 + 1] = "";
42008490:	d202                	sw	zero,36(sp)
42008492:	d402                	sw	zero,40(sp)
42008494:	d602                	sw	zero,44(sp)
42008496:	d802                	sw	zero,48(sp)
42008498:	da02                	sw	zero,52(sp)
4200849a:	dc02                	sw	zero,56(sp)
4200849c:	02010e23          	sb	zero,60(sp)
# 		for (int i = 0; i < p.len; i++) {
420084a0:	4401                	li	s0,0
420084a2:	bfad                	j	4200841c <cmd_dcc_sniff+0xb8>
# 	       printed, stats.packets_ok - ok_before);
420084a4:	3fc997b7          	lui	a5,0x3fc99
420084a8:	9a478793          	add	a5,a5,-1628 # 3fc989a4 <stats>
420084ac:	43d0                	lw	a2,4(a5)
# 	printf("Done. %u printed, %" PRIu32 " valid packets received.\n",
420084ae:	41460633          	sub	a2,a2,s4
420084b2:	85ce                	mv	a1,s3
420084b4:	3c075537          	lui	a0,0x3c075
420084b8:	bdc50513          	add	a0,a0,-1060 # 3c074bdc <_flash_rodata_start+0x4abc>
420084bc:	247520ef          	jal	4205af02 <printf>
# }
420084c0:	4501                	li	a0,0
420084c2:	50ba                	lw	ra,172(sp)
420084c4:	542a                	lw	s0,168(sp)
420084c6:	549a                	lw	s1,164(sp)
420084c8:	590a                	lw	s2,160(sp)
420084ca:	49fa                	lw	s3,156(sp)
420084cc:	4a6a                	lw	s4,152(sp)
420084ce:	614d                	add	sp,sp,176
420084d0:	8082                	ret

420084d2 <cmd_dcc>:
# 
# static int cmd_dcc(int argc, char **argv)
# {
420084d2:	1101                	add	sp,sp,-32
420084d4:	ce06                	sw	ra,28(sp)
420084d6:	cc22                	sw	s0,24(sp)
420084d8:	ca26                	sw	s1,20(sp)
420084da:	c84a                	sw	s2,16(sp)
420084dc:	c64e                	sw	s3,12(sp)
420084de:	c452                	sw	s4,8(sp)
420084e0:	c256                	sw	s5,4(sp)
420084e2:	84aa                	mv	s1,a0
420084e4:	89ae                	mv	s3,a1
# 	if (argc < 2) {
420084e6:	4785                	li	a5,1
420084e8:	06a7d063          	bge	a5,a0,42008548 <cmd_dcc+0x76>
# 		printf("usage: dcc <sniff [seconds] [-a] | stats [clear] | timing [ms]>\n");
# 		return 1;
# 	}
# 
# 	if (strcmp(argv[1], "sniff") == 0) {
420084ec:	41c0                	lw	s0,4(a1)
420084ee:	3c0755b7          	lui	a1,0x3c075
420084f2:	c4c58593          	add	a1,a1,-948 # 3c074c4c <_flash_rodata_start+0x4b2c>
420084f6:	8522                	mv	a0,s0
420084f8:	fdff8097          	auipc	ra,0xfdff8
420084fc:	e74080e7          	jalr	-396(ra) # 4000036c <strcmp>
42008500:	c551                	beqz	a0,4200858c <cmd_dcc+0xba>
# 		if (seconds <= 0) {
# 			printf("bad duration\n");
# 			return 1;
# 		}
# 		return cmd_dcc_sniff(seconds, all);
# 	} else if (strcmp(argv[1], "stats") == 0) {
42008502:	3c0755b7          	lui	a1,0x3c075
42008506:	c6858593          	add	a1,a1,-920 # 3c074c68 <_flash_rodata_start+0x4b48>
4200850a:	8522                	mv	a0,s0
4200850c:	fdff8097          	auipc	ra,0xfdff8
42008510:	e60080e7          	jalr	-416(ra) # 4000036c <strcmp>
42008514:	892a                	mv	s2,a0
42008516:	c555                	beqz	a0,420085c2 <cmd_dcc+0xf0>
# 		return cmd_dcc_stats(argc >= 3 && strcmp(argv[2], "clear") == 0);
# 	} else if (strcmp(argv[1], "timing") == 0) {
42008518:	3c0755b7          	lui	a1,0x3c075
4200851c:	c7858593          	add	a1,a1,-904 # 3c074c78 <_flash_rodata_start+0x4b58>
42008520:	8522                	mv	a0,s0
42008522:	fdff8097          	auipc	ra,0xfdff8
42008526:	e4a080e7          	jalr	-438(ra) # 4000036c <strcmp>
4200852a:	e979                	bnez	a0,42008600 <cmd_dcc+0x12e>
# 		int ms = argc >= 3 ? atoi(argv[2]) : DCC_TIMING_DEFAULT_MS;
4200852c:	4789                	li	a5,2
4200852e:	0c97d663          	bge	a5,s1,420085fa <cmd_dcc+0x128>
42008532:	0089a503          	lw	a0,8(s3)
42008536:	fdff8097          	auipc	ra,0xfdff8
4200853a:	f16080e7          	jalr	-234(ra) # 4000044c <atoi>
# 		if (ms <= 0) {
4200853e:	0aa05663          	blez	a0,420085ea <cmd_dcc+0x118>
# 			printf("bad duration\n");
# 			return 1;
# 		}
# 		return cmd_dcc_timing(ms);
42008542:	b7fff0ef          	jal	420080c0 <cmd_dcc_timing>
42008546:	a8a9                	j	420085a0 <cmd_dcc+0xce>
# 		printf("usage: dcc <sniff [seconds] [-a] | stats [clear] | timing [ms]>\n");
42008548:	3c075537          	lui	a0,0x3c075
4200854c:	c0c50513          	add	a0,a0,-1012 # 3c074c0c <_flash_rodata_start+0x4aec>
42008550:	347520ef          	jal	4205b096 <puts>
# 		return 1;
42008554:	4505                	li	a0,1
42008556:	a0a9                	j	420085a0 <cmd_dcc+0xce>
# 				seconds = atoi(argv[i]);
42008558:	854a                	mv	a0,s2
4200855a:	fdff8097          	auipc	ra,0xfdff8
4200855e:	ef2080e7          	jalr	-270(ra) # 4000044c <atoi>
42008562:	8a2a                	mv	s4,a0
# 		for (int i = 2; i < argc; i++) {
42008564:	0405                	add	s0,s0,1
42008566:	02945763          	bge	s0,s1,42008594 <cmd_dcc+0xc2>
# 			if (strcmp(argv[i], "-a") == 0) {
4200856a:	00241793          	sll	a5,s0,0x2
4200856e:	97ce                	add	a5,a5,s3
42008570:	0007a903          	lw	s2,0(a5)
42008574:	3c0755b7          	lui	a1,0x3c075
42008578:	c5458593          	add	a1,a1,-940 # 3c074c54 <_flash_rodata_start+0x4b34>
4200857c:	854a                	mv	a0,s2
4200857e:	fdff8097          	auipc	ra,0xfdff8
42008582:	dee080e7          	jalr	-530(ra) # 4000036c <strcmp>
42008586:	f969                	bnez	a0,42008558 <cmd_dcc+0x86>
# 				all = true;
42008588:	4a85                	li	s5,1
4200858a:	bfe9                	j	42008564 <cmd_dcc+0x92>
# 		for (int i = 2; i < argc; i++) {
4200858c:	4409                	li	s0,2
# 		bool all = false;
4200858e:	4a81                	li	s5,0
# 		int seconds = DCC_SNIFF_DEFAULT_S;
42008590:	4a79                	li	s4,30
42008592:	bfd1                	j	42008566 <cmd_dcc+0x94>
# 		if (seconds <= 0) {
42008594:	01405f63          	blez	s4,420085b2 <cmd_dcc+0xe0>
# 		return cmd_dcc_sniff(seconds, all);
42008598:	85d6                	mv	a1,s5
4200859a:	8552                	mv	a0,s4
4200859c:	dc9ff0ef          	jal	42008364 <cmd_dcc_sniff>
# 	}
# 
# 	printf("unknown dcc subcommand: '%s'\n", argv[1]);
# 	return 1;
# }
420085a0:	40f2                	lw	ra,28(sp)
420085a2:	4462                	lw	s0,24(sp)
420085a4:	44d2                	lw	s1,20(sp)
420085a6:	4942                	lw	s2,16(sp)
420085a8:	49b2                	lw	s3,12(sp)
420085aa:	4a22                	lw	s4,8(sp)
420085ac:	4a92                	lw	s5,4(sp)
420085ae:	6105                	add	sp,sp,32
420085b0:	8082                	ret
# 			printf("bad duration\n");
420085b2:	3c075537          	lui	a0,0x3c075
420085b6:	c5850513          	add	a0,a0,-936 # 3c074c58 <_flash_rodata_start+0x4b38>
420085ba:	2dd520ef          	jal	4205b096 <puts>
# 			return 1;
420085be:	4505                	li	a0,1
420085c0:	b7c5                	j	420085a0 <cmd_dcc+0xce>
# 		return cmd_dcc_stats(argc >= 3 && strcmp(argv[2], "clear") == 0);
420085c2:	4789                	li	a5,2
420085c4:	0097de63          	bge	a5,s1,420085e0 <cmd_dcc+0x10e>
420085c8:	3c0755b7          	lui	a1,0x3c075
420085cc:	c7058593          	add	a1,a1,-912 # 3c074c70 <_flash_rodata_start+0x4b50>
420085d0:	0089a503          	lw	a0,8(s3)
420085d4:	fdff8097          	auipc	ra,0xfdff8
420085d8:	d98080e7          	jalr	-616(ra) # 4000036c <strcmp>
420085dc:	e111                	bnez	a0,420085e0 <cmd_dcc+0x10e>
420085de:	4905                	li	s2,1
420085e0:	00197513          	and	a0,s2,1
420085e4:	903ff0ef          	jal	42007ee6 <cmd_dcc_stats>
420085e8:	bf65                	j	420085a0 <cmd_dcc+0xce>
# 			printf("bad duration\n");
420085ea:	3c075537          	lui	a0,0x3c075
420085ee:	c5850513          	add	a0,a0,-936 # 3c074c58 <_flash_rodata_start+0x4b38>
420085f2:	2a5520ef          	jal	4205b096 <puts>
# 			return 1;
420085f6:	4505                	li	a0,1
420085f8:	b765                	j	420085a0 <cmd_dcc+0xce>
# 		int ms = argc >= 3 ? atoi(argv[2]) : DCC_TIMING_DEFAULT_MS;
420085fa:	3e800513          	li	a0,1000
420085fe:	b791                	j	42008542 <cmd_dcc+0x70>
# 	printf("unknown dcc subcommand: '%s'\n", argv[1]);
42008600:	85a2                	mv	a1,s0
42008602:	3c075537          	lui	a0,0x3c075
42008606:	c8050513          	add	a0,a0,-896 # 3c074c80 <_flash_rodata_start+0x4b60>
4200860a:	0f9520ef          	jal	4205af02 <printf>
# 	return 1;
4200860e:	4505                	li	a0,1
42008610:	bf41                	j	420085a0 <cmd_dcc+0xce>

42008612 <dcc_register_cli_commands>:
# 
# void dcc_register_cli_commands(void)
# {
42008612:	7179                	add	sp,sp,-48
42008614:	d606                	sw	ra,44(sp)
# 	const esp_console_cmd_t dcc_cmd = {
42008616:	c202                	sw	zero,4(sp)
42008618:	c402                	sw	zero,8(sp)
4200861a:	c602                	sw	zero,12(sp)
4200861c:	c802                	sw	zero,16(sp)
4200861e:	ca02                	sw	zero,20(sp)
42008620:	cc02                	sw	zero,24(sp)
42008622:	ce02                	sw	zero,28(sp)
42008624:	3c0757b7          	lui	a5,0x3c075
42008628:	ca078793          	add	a5,a5,-864 # 3c074ca0 <_flash_rodata_start+0x4b80>
4200862c:	c23e                	sw	a5,4(sp)
4200862e:	3c0757b7          	lui	a5,0x3c075
42008632:	ca478793          	add	a5,a5,-860 # 3c074ca4 <_flash_rodata_start+0x4b84>
42008636:	c43e                	sw	a5,8(sp)
42008638:	420087b7          	lui	a5,0x42008
4200863c:	4d278793          	add	a5,a5,1234 # 420084d2 <cmd_dcc>
42008640:	c83e                	sw	a5,16(sp)
# 		.command = "dcc",
# 		.help = "DCC: sniff [seconds] [-a] | stats [clear] | timing [ms]",
# 		.func = &cmd_dcc,
# 	};
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&dcc_cmd));
42008642:	0048                	add	a0,sp,4
42008644:	173010ef          	jal	42009fb6 <esp_console_cmd_register>
42008648:	e501                	bnez	a0,42008650 <dcc_register_cli_commands+0x3e>
# }
4200864a:	50b2                	lw	ra,44(sp)
4200864c:	6145                	add	sp,sp,48
4200864e:	8082                	ret
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&dcc_cmd));
42008650:	3c075737          	lui	a4,0x3c075
42008654:	cdc70713          	add	a4,a4,-804 # 3c074cdc <_flash_rodata_start+0x4bbc>
42008658:	3c07e6b7          	lui	a3,0x3c07e
4200865c:	a4468693          	add	a3,a3,-1468 # 3c07da44 <__func__.1>
42008660:	23400613          	li	a2,564
42008664:	3c0755b7          	lui	a1,0x3c075
42008668:	d0058593          	add	a1,a1,-768 # 3c074d00 <_flash_rodata_start+0x4be0>
4200866c:	fe384097          	auipc	ra,0xfe384
42008670:	382080e7          	jalr	898(ra) # 4038c9ee <_esp_error_check_failed>

42008674 <dcc_init>:
# 
# void dcc_init(void)
# {
42008674:	7179                	add	sp,sp,-48
42008676:	d606                	sw	ra,44(sp)
# 	dcc_rx_queue = xQueueCreate(DCC_RX_QUEUE_LEN, sizeof(dcc_packet_t));
42008678:	4601                	li	a2,0
4200867a:	45e1                	li	a1,24
4200867c:	04000513          	li	a0,64
42008680:	fe385097          	auipc	ra,0xfe385
42008684:	82a080e7          	jalr	-2006(ra) # 4038ceaa <xQueueGenericCreate>
42008688:	3fc997b7          	lui	a5,0x3fc99
4200868c:	36a7a423          	sw	a0,872(a5) # 3fc99368 <dcc_rx_queue>
# 
# 	/* The internal pull-up (~45k) sits alongside the external 2.2k one;
# 	 * it only matters with the opto unplugged, where it keeps the pin
# 	 * from floating and firing spurious interrupts. */
# 	const gpio_config_t io_conf = {
42008690:	3c07e7b7          	lui	a5,0x3c07e
42008694:	ba078793          	add	a5,a5,-1120 # 3c07dba0 <__func__.1+0x15c>
42008698:	4388                	lw	a0,0(a5)
4200869a:	43cc                	lw	a1,4(a5)
4200869c:	4790                	lw	a2,8(a5)
4200869e:	47d4                	lw	a3,12(a5)
420086a0:	4b98                	lw	a4,16(a5)
420086a2:	4bdc                	lw	a5,20(a5)
420086a4:	c42a                	sw	a0,8(sp)
420086a6:	c62e                	sw	a1,12(sp)
420086a8:	c832                	sw	a2,16(sp)
420086aa:	ca36                	sw	a3,20(sp)
420086ac:	cc3a                	sw	a4,24(sp)
420086ae:	ce3e                	sw	a5,28(sp)
# 		.mode = GPIO_MODE_INPUT,
# 		.pull_up_en = GPIO_PULLUP_ENABLE,
# 		.pull_down_en = GPIO_PULLDOWN_DISABLE,
# 		.intr_type = GPIO_INTR_ANYEDGE,
# 	};
# 	ESP_ERROR_CHECK(gpio_config(&io_conf));
420086b0:	0028                	add	a0,sp,8
420086b2:	79d180ef          	jal	4202164e <gpio_config>
420086b6:	e125                	bnez	a0,42008716 <dcc_init+0xa2>
# 
# 	last_edge_us = esp_timer_get_time();
420086b8:	fe378097          	auipc	ra,0xfe378
420086bc:	c6c080e7          	jalr	-916(ra) # 40380324 <esp_timer_get_time>
420086c0:	3fc997b7          	lui	a5,0x3fc99
420086c4:	36a7a023          	sw	a0,864(a5) # 3fc99360 <last_edge_us>
420086c8:	36b7a223          	sw	a1,868(a5)
# 	ESP_ERROR_CHECK(gpio_install_isr_service(0));
420086cc:	4501                	li	a0,0
420086ce:	3c4190ef          	jal	42021a92 <gpio_install_isr_service>
420086d2:	e525                	bnez	a0,4200873a <dcc_init+0xc6>
# 	ESP_ERROR_CHECK(gpio_isr_handler_add(DCC_IN_GPIO, dcc_edge_isr, NULL));
420086d4:	4601                	li	a2,0
420086d6:	403825b7          	lui	a1,0x40382
420086da:	d1c58593          	add	a1,a1,-740 # 40381d1c <dcc_edge_isr>
420086de:	4511                	li	a0,4
420086e0:	194190ef          	jal	42021874 <gpio_isr_handler_add>
420086e4:	ed2d                	bnez	a0,4200875e <dcc_init+0xea>
# 
# 	ESP_LOGI(TAG, "listening on GPIO%d", DCC_IN_GPIO);
420086e6:	fe38b097          	auipc	ra,0xfe38b
420086ea:	ff6080e7          	jalr	-10(ra) # 403936dc <esp_log_timestamp>
420086ee:	86aa                	mv	a3,a0
420086f0:	4791                	li	a5,4
420086f2:	3c0755b7          	lui	a1,0x3c075
420086f6:	ca058713          	add	a4,a1,-864 # 3c074ca0 <_flash_rodata_start+0x4b80>
420086fa:	3c075637          	lui	a2,0x3c075
420086fe:	d7c60613          	add	a2,a2,-644 # 3c074d7c <_flash_rodata_start+0x4c5c>
42008702:	ca058593          	add	a1,a1,-864
42008706:	450d                	li	a0,3
42008708:	fe38b097          	auipc	ra,0xfe38b
4200870c:	ee6080e7          	jalr	-282(ra) # 403935ee <esp_log_write>
# }
42008710:	50b2                	lw	ra,44(sp)
42008712:	6145                	add	sp,sp,48
42008714:	8082                	ret
# 	ESP_ERROR_CHECK(gpio_config(&io_conf));
42008716:	3c075737          	lui	a4,0x3c075
4200871a:	d1070713          	add	a4,a4,-752 # 3c074d10 <_flash_rodata_start+0x4bf0>
4200871e:	3c07e6b7          	lui	a3,0x3c07e
42008722:	a3868693          	add	a3,a3,-1480 # 3c07da38 <__func__.0>
42008726:	24500613          	li	a2,581
4200872a:	3c0755b7          	lui	a1,0x3c075
4200872e:	d0058593          	add	a1,a1,-768 # 3c074d00 <_flash_rodata_start+0x4be0>
42008732:	fe384097          	auipc	ra,0xfe384
42008736:	2bc080e7          	jalr	700(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(gpio_install_isr_service(0));
4200873a:	3c075737          	lui	a4,0x3c075
4200873e:	d2870713          	add	a4,a4,-728 # 3c074d28 <_flash_rodata_start+0x4c08>
42008742:	3c07e6b7          	lui	a3,0x3c07e
42008746:	a3868693          	add	a3,a3,-1480 # 3c07da38 <__func__.0>
4200874a:	24800613          	li	a2,584
4200874e:	3c0755b7          	lui	a1,0x3c075
42008752:	d0058593          	add	a1,a1,-768 # 3c074d00 <_flash_rodata_start+0x4be0>
42008756:	fe384097          	auipc	ra,0xfe384
4200875a:	298080e7          	jalr	664(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(gpio_isr_handler_add(DCC_IN_GPIO, dcc_edge_isr, NULL));
4200875e:	3c075737          	lui	a4,0x3c075
42008762:	d4470713          	add	a4,a4,-700 # 3c074d44 <_flash_rodata_start+0x4c24>
42008766:	3c07e6b7          	lui	a3,0x3c07e
4200876a:	a3868693          	add	a3,a3,-1480 # 3c07da38 <__func__.0>
4200876e:	24900613          	li	a2,585
42008772:	3c0755b7          	lui	a1,0x3c075
42008776:	d0058593          	add	a1,a1,-768 # 3c074d00 <_flash_rodata_start+0x4be0>
4200877a:	fe384097          	auipc	ra,0xfe384
4200877e:	274080e7          	jalr	628(ra) # 4038c9ee <_esp_error_check_failed>

