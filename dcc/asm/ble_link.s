# ble_link.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

42008782 <gatt_access>:
#  * NimBLE never actually calls this. */
# static int gatt_access(uint16_t conn, uint16_t attr,
# 		       struct ble_gatt_access_ctxt *ctxt, void *arg)
# {
# 	return BLE_ATT_ERR_UNLIKELY;
# }
42008782:	4539                	li	a0,14
42008784:	8082                	ret

42008786 <host_task>:
# {
# 	ESP_LOGW(TAG, "host reset, reason %d", reason);
# }
# 
# static void host_task(void *arg)
# {
42008786:	1141                	add	sp,sp,-16
42008788:	c606                	sw	ra,12(sp)
# 	nimble_port_run();  /* returns only on nimble_port_stop() */
4200878a:	fe379097          	auipc	ra,0xfe379
4200878e:	6ea080e7          	jalr	1770(ra) # 40381e74 <nimble_port_run>
# 	nimble_port_freertos_deinit();
42008792:	7b20d0ef          	jal	42015f44 <nimble_port_freertos_deinit>
# }
42008796:	40b2                	lw	ra,12(sp)
42008798:	0141                	add	sp,sp,16
4200879a:	8082                	ret

4200879c <on_leds_changed>:
# {
4200879c:	1141                	add	sp,sp,-16
4200879e:	c606                	sw	ra,12(sp)
# 	ble_gatts_chr_updated(leds_handle);
420087a0:	3fc997b7          	lui	a5,0x3fc99
420087a4:	3767d503          	lhu	a0,886(a5) # 3fc99376 <leds_handle>
420087a8:	1f40c0ef          	jal	4201499c <ble_gatts_chr_updated>
# }
420087ac:	40b2                	lw	ra,12(sp)
420087ae:	0141                	add	sp,sp,16
420087b0:	8082                	ret

420087b2 <leds_access>:
# {
420087b2:	7135                	add	sp,sp,-160
420087b4:	cf06                	sw	ra,156(sp)
420087b6:	cd22                	sw	s0,152(sp)
420087b8:	cb26                	sw	s1,148(sp)
420087ba:	c94a                	sw	s2,144(sp)
420087bc:	84b2                	mv	s1,a2
# 	if (ctxt->op == BLE_GATT_ACCESS_OP_READ_CHR) {
420087be:	00064403          	lbu	s0,0(a2)
420087c2:	cc01                	beqz	s0,420087da <leds_access+0x28>
# 	if (ctxt->op == BLE_GATT_ACCESS_OP_WRITE_CHR) {
420087c4:	4785                	li	a5,1
420087c6:	08f40663          	beq	s0,a5,42008852 <leds_access+0xa0>
# 	return BLE_ATT_ERR_UNLIKELY;
420087ca:	4439                	li	s0,14
# }
420087cc:	8522                	mv	a0,s0
420087ce:	40fa                	lw	ra,156(sp)
420087d0:	446a                	lw	s0,152(sp)
420087d2:	44da                	lw	s1,148(sp)
420087d4:	494a                	lw	s2,144(sp)
420087d6:	610d                	add	sp,sp,160
420087d8:	8082                	ret
# 		uint8_t n = leds_count();
420087da:	224010ef          	jal	420099fe <leds_count>
420087de:	892a                	mv	s2,a0
# 		if (n > 32) {
420087e0:	02000793          	li	a5,32
420087e4:	00a7f463          	bgeu	a5,a0,420087ec <leds_access+0x3a>
# 			n = 32;
420087e8:	02000913          	li	s2,32
# 		buf[0] = n;
420087ec:	01210623          	sb	s2,12(sp)
# 		buf[1] = leds_brightness();
420087f0:	212010ef          	jal	42009a02 <leds_brightness>
420087f4:	00a106a3          	sb	a0,13(sp)
# 		for (uint8_t i = 0; i < n; i++) {
420087f8:	a82d                	j	42008832 <leds_access+0x80>
# 			led_state_t s = leds_get(i);
420087fa:	8522                	mv	a0,s0
420087fc:	20c010ef          	jal	42009a08 <leds_get>
42008800:	c42a                	sw	a0,8(sp)
# 			uint8_t *p = buf + 2 + i * 4;
42008802:	00241793          	sll	a5,s0,0x2
42008806:	0789                	add	a5,a5,2
42008808:	0078                	add	a4,sp,12
4200880a:	97ba                	add	a5,a5,a4
# 			p[0] = s.on;
4200880c:	00814703          	lbu	a4,8(sp)
42008810:	00e78023          	sb	a4,0(a5)
# 			p[1] = s.r;
42008814:	00914703          	lbu	a4,9(sp)
42008818:	00e780a3          	sb	a4,1(a5)
# 			p[2] = s.g;
4200881c:	00a14703          	lbu	a4,10(sp)
42008820:	00e78123          	sb	a4,2(a5)
# 			p[3] = s.b;
42008824:	00b14703          	lbu	a4,11(sp)
42008828:	00e781a3          	sb	a4,3(a5)
# 		for (uint8_t i = 0; i < n; i++) {
4200882c:	0405                	add	s0,s0,1
4200882e:	0ff47413          	zext.b	s0,s0
42008832:	fd2464e3          	bltu	s0,s2,420087fa <leds_access+0x48>
# 		return os_mbuf_append(ctxt->om, buf, 2 + n * 4) == 0 ? 0 : BLE_ATT_ERR_INSUFFICIENT_RES;
42008836:	00291793          	sll	a5,s2,0x2
4200883a:	0789                	add	a5,a5,2
4200883c:	01079613          	sll	a2,a5,0x10
42008840:	8241                	srl	a2,a2,0x10
42008842:	006c                	add	a1,sp,12
42008844:	40c8                	lw	a0,4(s1)
42008846:	4090d0ef          	jal	4201644e <os_mbuf_append>
4200884a:	842a                	mv	s0,a0
4200884c:	d141                	beqz	a0,420087cc <leds_access+0x1a>
4200884e:	4445                	li	s0,17
42008850:	bfb5                	j	420087cc <leds_access+0x1a>
# 		uint16_t len = 0;
42008852:	00011423          	sh	zero,8(sp)
# 		if (ble_hs_mbuf_to_flat(ctxt->om, cmd, sizeof(cmd), &len) != 0 || len < 2) {
42008856:	0034                	add	a3,sp,8
42008858:	4615                	li	a2,5
4200885a:	006c                	add	a1,sp,12
4200885c:	40c8                	lw	a0,4(s1)
4200885e:	4f2070ef          	jal	4200fd50 <ble_hs_mbuf_to_flat>
42008862:	842a                	mv	s0,a0
42008864:	e159                	bnez	a0,420088ea <leds_access+0x138>
42008866:	00815483          	lhu	s1,8(sp)
4200886a:	4785                	li	a5,1
4200886c:	0897f163          	bgeu	a5,s1,420088ee <leds_access+0x13c>
# 		switch (cmd[0]) {
42008870:	00c14783          	lbu	a5,12(sp)
42008874:	4709                	li	a4,2
42008876:	02e78863          	beq	a5,a4,420088a6 <leds_access+0xf4>
4200887a:	470d                	li	a4,3
4200887c:	04e78863          	beq	a5,a4,420088cc <leds_access+0x11a>
42008880:	4705                	li	a4,1
42008882:	06e79263          	bne	a5,a4,420088e6 <leds_access+0x134>
# 			ok = len == 3 && leds_set_on(cmd[1], cmd[2] != 0);
42008886:	478d                	li	a5,3
42008888:	00f48463          	beq	s1,a5,42008890 <leds_access+0xde>
# 		return ok ? 0 : BLE_ATT_ERR_VALUE_NOT_ALLOWED;
4200888c:	444d                	li	s0,19
4200888e:	bf3d                	j	420087cc <leds_access+0x1a>
# 			ok = len == 3 && leds_set_on(cmd[1], cmd[2] != 0);
42008890:	00e14583          	lbu	a1,14(sp)
42008894:	00b035b3          	snez	a1,a1
42008898:	00d14503          	lbu	a0,13(sp)
4200889c:	767000ef          	jal	42009802 <leds_set_on>
420088a0:	f515                	bnez	a0,420087cc <leds_access+0x1a>
# 		return ok ? 0 : BLE_ATT_ERR_VALUE_NOT_ALLOWED;
420088a2:	444d                	li	s0,19
420088a4:	b725                	j	420087cc <leds_access+0x1a>
# 			ok = len == 5 && leds_set_color(cmd[1], cmd[2], cmd[3], cmd[4]);
420088a6:	4795                	li	a5,5
420088a8:	00f48463          	beq	s1,a5,420088b0 <leds_access+0xfe>
# 		return ok ? 0 : BLE_ATT_ERR_VALUE_NOT_ALLOWED;
420088ac:	444d                	li	s0,19
420088ae:	bf39                	j	420087cc <leds_access+0x1a>
# 			ok = len == 5 && leds_set_color(cmd[1], cmd[2], cmd[3], cmd[4]);
420088b0:	01014683          	lbu	a3,16(sp)
420088b4:	00f14603          	lbu	a2,15(sp)
420088b8:	00e14583          	lbu	a1,14(sp)
420088bc:	00d14503          	lbu	a0,13(sp)
420088c0:	7cb000ef          	jal	4200988a <leds_set_color>
420088c4:	f00514e3          	bnez	a0,420087cc <leds_access+0x1a>
# 		return ok ? 0 : BLE_ATT_ERR_VALUE_NOT_ALLOWED;
420088c8:	444d                	li	s0,19
420088ca:	b709                	j	420087cc <leds_access+0x1a>
# 			if (ok) {
420088cc:	4789                	li	a5,2
420088ce:	00f48763          	beq	s1,a5,420088dc <leds_access+0x12a>
# 		return ok ? 0 : BLE_ATT_ERR_VALUE_NOT_ALLOWED;
420088d2:	4789                	li	a5,2
420088d4:	eef48ce3          	beq	s1,a5,420087cc <leds_access+0x1a>
420088d8:	444d                	li	s0,19
420088da:	bdcd                	j	420087cc <leds_access+0x1a>
# 				leds_set_brightness(cmd[1]);
420088dc:	00d14503          	lbu	a0,13(sp)
420088e0:	0da010ef          	jal	420099ba <leds_set_brightness>
420088e4:	b7fd                	j	420088d2 <leds_access+0x120>
# 		switch (cmd[0]) {
420088e6:	444d                	li	s0,19
420088e8:	b5d5                	j	420087cc <leds_access+0x1a>
# 			return BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN;
420088ea:	4435                	li	s0,13
420088ec:	b5c5                	j	420087cc <leds_access+0x1a>
420088ee:	4435                	li	s0,13
420088f0:	bdf1                	j	420087cc <leds_access+0x1a>

420088f2 <on_reset>:
# {
420088f2:	1141                	add	sp,sp,-16
420088f4:	c606                	sw	ra,12(sp)
420088f6:	c422                	sw	s0,8(sp)
420088f8:	842a                	mv	s0,a0
# 	ESP_LOGW(TAG, "host reset, reason %d", reason);
420088fa:	fe38b097          	auipc	ra,0xfe38b
420088fe:	de2080e7          	jalr	-542(ra) # 403936dc <esp_log_timestamp>
42008902:	86aa                	mv	a3,a0
42008904:	87a2                	mv	a5,s0
42008906:	3c0735b7          	lui	a1,0x3c073
4200890a:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
4200890e:	3c075637          	lui	a2,0x3c075
42008912:	da860613          	add	a2,a2,-600 # 3c074da8 <_flash_rodata_start+0x4c88>
42008916:	0a458593          	add	a1,a1,164
4200891a:	4509                	li	a0,2
4200891c:	fe38b097          	auipc	ra,0xfe38b
42008920:	cd2080e7          	jalr	-814(ra) # 403935ee <esp_log_write>
# }
42008924:	40b2                	lw	ra,12(sp)
42008926:	4422                	lw	s0,8(sp)
42008928:	0141                	add	sp,sp,16
4200892a:	8082                	ret

4200892c <advertise>:
# {
4200892c:	714d                	add	sp,sp,-336
4200892e:	14112623          	sw	ra,332(sp)
42008932:	14812423          	sw	s0,328(sp)
# 	struct ble_hs_adv_fields fields = {
42008936:	09400613          	li	a2,148
4200893a:	4581                	li	a1,0
4200893c:	1168                	add	a0,sp,172
4200893e:	fdff8097          	auipc	ra,0xfdff8
42008942:	a16080e7          	jalr	-1514(ra) # 40000354 <memset>
42008946:	4799                	li	a5,6
42008948:	0af10623          	sb	a5,172(sp)
4200894c:	3c07e7b7          	lui	a5,0x3c07e
42008950:	c3078793          	add	a5,a5,-976 # 3c07dc30 <svc_uuid>
42008954:	c1be                	sw	a5,192(sp)
42008956:	4785                	li	a5,1
42008958:	0cf10223          	sb	a5,196(sp)
4200895c:	0c514783          	lbu	a5,197(sp)
42008960:	0017e793          	or	a5,a5,1
42008964:	0cf102a3          	sb	a5,197(sp)
# 	int rc = ble_gap_adv_set_fields(&fields);
42008968:	1168                	add	a0,sp,172
4200896a:	0a8090ef          	jal	42011a12 <ble_gap_adv_set_fields>
4200896e:	842a                	mv	s0,a0
# 	if (rc != 0) {
42008970:	ed49                	bnez	a0,42008a0a <advertise+0xde>
# 	struct ble_hs_adv_fields rsp = {
42008972:	09400613          	li	a2,148
42008976:	4581                	li	a1,0
42008978:	0828                	add	a0,sp,24
4200897a:	fdff8097          	auipc	ra,0xfdff8
4200897e:	9da080e7          	jalr	-1574(ra) # 40000354 <memset>
42008982:	3c0757b7          	lui	a5,0x3c075
42008986:	e0078793          	add	a5,a5,-512 # 3c074e00 <_flash_rodata_start+0x4ce0>
4200898a:	da3e                	sw	a5,52(sp)
4200898c:	47ad                	li	a5,11
4200898e:	02f10c23          	sb	a5,56(sp)
42008992:	03914783          	lbu	a5,57(sp)
42008996:	0017e793          	or	a5,a5,1
4200899a:	02f10ca3          	sb	a5,57(sp)
# 	rc = ble_gap_adv_rsp_set_fields(&rsp);
4200899e:	0828                	add	a0,sp,24
420089a0:	0bc090ef          	jal	42011a5c <ble_gap_adv_rsp_set_fields>
420089a4:	842a                	mv	s0,a0
# 	if (rc != 0) {
420089a6:	ed49                	bnez	a0,42008a40 <advertise+0x114>
# 	struct ble_gap_adv_params params = {
420089a8:	c602                	sw	zero,12(sp)
420089aa:	c802                	sw	zero,16(sp)
420089ac:	00011a23          	sh	zero,20(sp)
420089b0:	4789                	li	a5,2
420089b2:	00f10623          	sb	a5,12(sp)
420089b6:	00f106a3          	sb	a5,13(sp)
# 	rc = ble_gap_adv_start(own_addr_type, NULL, BLE_HS_FOREVER, &params,
420089ba:	4781                	li	a5,0
420089bc:	42009737          	lui	a4,0x42009
420089c0:	abe70713          	add	a4,a4,-1346 # 42008abe <gap_event>
420089c4:	0074                	add	a3,sp,12
420089c6:	80000637          	lui	a2,0x80000
420089ca:	167d                	add	a2,a2,-1 # 7fffffff <SYSTEM+0x1ff3ffff>
420089cc:	4581                	li	a1,0
420089ce:	3fc99537          	lui	a0,0x3fc99
420089d2:	37554503          	lbu	a0,885(a0) # 3fc99375 <own_addr_type>
420089d6:	599080ef          	jal	4201176e <ble_gap_adv_start>
420089da:	842a                	mv	s0,a0
# 	if (rc != 0) {
420089dc:	cd21                	beqz	a0,42008a34 <advertise+0x108>
# 		ESP_LOGE(TAG, "adv start: %d", rc);
420089de:	fe38b097          	auipc	ra,0xfe38b
420089e2:	cfe080e7          	jalr	-770(ra) # 403936dc <esp_log_timestamp>
420089e6:	86aa                	mv	a3,a0
420089e8:	87a2                	mv	a5,s0
420089ea:	3c0735b7          	lui	a1,0x3c073
420089ee:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
420089f2:	3c075637          	lui	a2,0x3c075
420089f6:	e4060613          	add	a2,a2,-448 # 3c074e40 <_flash_rodata_start+0x4d20>
420089fa:	0a458593          	add	a1,a1,164
420089fe:	4505                	li	a0,1
42008a00:	fe38b097          	auipc	ra,0xfe38b
42008a04:	bee080e7          	jalr	-1042(ra) # 403935ee <esp_log_write>
42008a08:	a035                	j	42008a34 <advertise+0x108>
# 		ESP_LOGE(TAG, "adv fields: %d", rc);
42008a0a:	fe38b097          	auipc	ra,0xfe38b
42008a0e:	cd2080e7          	jalr	-814(ra) # 403936dc <esp_log_timestamp>
42008a12:	86aa                	mv	a3,a0
42008a14:	87a2                	mv	a5,s0
42008a16:	3c0735b7          	lui	a1,0x3c073
42008a1a:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008a1e:	3c075637          	lui	a2,0x3c075
42008a22:	dd860613          	add	a2,a2,-552 # 3c074dd8 <_flash_rodata_start+0x4cb8>
42008a26:	0a458593          	add	a1,a1,164
42008a2a:	4505                	li	a0,1
42008a2c:	fe38b097          	auipc	ra,0xfe38b
42008a30:	bc2080e7          	jalr	-1086(ra) # 403935ee <esp_log_write>
# }
42008a34:	14c12083          	lw	ra,332(sp)
42008a38:	14812403          	lw	s0,328(sp)
42008a3c:	6171                	add	sp,sp,336
42008a3e:	8082                	ret
# 		ESP_LOGE(TAG, "scan response fields: %d", rc);
42008a40:	fe38b097          	auipc	ra,0xfe38b
42008a44:	c9c080e7          	jalr	-868(ra) # 403936dc <esp_log_timestamp>
42008a48:	86aa                	mv	a3,a0
42008a4a:	87a2                	mv	a5,s0
42008a4c:	3c0735b7          	lui	a1,0x3c073
42008a50:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008a54:	3c075637          	lui	a2,0x3c075
42008a58:	e0c60613          	add	a2,a2,-500 # 3c074e0c <_flash_rodata_start+0x4cec>
42008a5c:	0a458593          	add	a1,a1,164
42008a60:	4505                	li	a0,1
42008a62:	fe38b097          	auipc	ra,0xfe38b
42008a66:	b8c080e7          	jalr	-1140(ra) # 403935ee <esp_log_write>
# 		return;
42008a6a:	b7e9                	j	42008a34 <advertise+0x108>

42008a6c <on_sync>:
# {
42008a6c:	1141                	add	sp,sp,-16
42008a6e:	c606                	sw	ra,12(sp)
# 	ble_hs_util_ensure_addr(0);
42008a70:	4501                	li	a0,0
42008a72:	156040ef          	jal	4200cbc8 <ble_hs_util_ensure_addr>
# 	ble_hs_id_infer_auto(0, &own_addr_type);
42008a76:	3fc995b7          	lui	a1,0x3fc99
42008a7a:	37558593          	add	a1,a1,885 # 3fc99375 <own_addr_type>
42008a7e:	4501                	li	a0,0
42008a80:	011040ef          	jal	4200d290 <ble_hs_id_infer_auto>
# 	advertise();
42008a84:	ea9ff0ef          	jal	4200892c <advertise>
# 	ESP_LOGI(TAG, "advertising as %s", BLE_DEVICE_NAME);
42008a88:	fe38b097          	auipc	ra,0xfe38b
42008a8c:	c54080e7          	jalr	-940(ra) # 403936dc <esp_log_timestamp>
42008a90:	86aa                	mv	a3,a0
42008a92:	3c0757b7          	lui	a5,0x3c075
42008a96:	e0078793          	add	a5,a5,-512 # 3c074e00 <_flash_rodata_start+0x4ce0>
42008a9a:	3c0735b7          	lui	a1,0x3c073
42008a9e:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008aa2:	3c075637          	lui	a2,0x3c075
42008aa6:	e6860613          	add	a2,a2,-408 # 3c074e68 <_flash_rodata_start+0x4d48>
42008aaa:	0a458593          	add	a1,a1,164
42008aae:	450d                	li	a0,3
42008ab0:	fe38b097          	auipc	ra,0xfe38b
42008ab4:	b3e080e7          	jalr	-1218(ra) # 403935ee <esp_log_write>
# }
42008ab8:	40b2                	lw	ra,12(sp)
42008aba:	0141                	add	sp,sp,16
42008abc:	8082                	ret

42008abe <gap_event>:
# {
42008abe:	1141                	add	sp,sp,-16
42008ac0:	c606                	sw	ra,12(sp)
42008ac2:	c422                	sw	s0,8(sp)
42008ac4:	842a                	mv	s0,a0
# 	switch (event->type) {
42008ac6:	00054783          	lbu	a5,0(a0)
42008aca:	4725                	li	a4,9
42008acc:	12e78063          	beq	a5,a4,42008bec <gap_event+0x12e>
42008ad0:	00f77d63          	bgeu	a4,a5,42008aea <gap_event+0x2c>
42008ad4:	4739                	li	a4,14
42008ad6:	0ae78963          	beq	a5,a4,42008b88 <gap_event+0xca>
42008ada:	473d                	li	a4,15
42008adc:	04e79c63          	bne	a5,a4,42008b34 <gap_event+0x76>
# 		mtu = event->mtu.value;
42008ae0:	00855703          	lhu	a4,8(a0)
42008ae4:	84e19623          	sh	a4,-1972(gp) # 3fc9524c <mtu>
# 		break;
42008ae8:	a0b1                	j	42008b34 <gap_event+0x76>
# 	switch (event->type) {
42008aea:	cbb1                	beqz	a5,42008b3e <gap_event+0x80>
42008aec:	4705                	li	a4,1
42008aee:	04e79363          	bne	a5,a4,42008b34 <gap_event+0x76>
# 		ESP_LOGI(TAG, "client disconnected (reason 0x%x)", event->disconnect.reason);
42008af2:	fe38b097          	auipc	ra,0xfe38b
42008af6:	bea080e7          	jalr	-1046(ra) # 403936dc <esp_log_timestamp>
42008afa:	86aa                	mv	a3,a0
42008afc:	405c                	lw	a5,4(s0)
42008afe:	3c0735b7          	lui	a1,0x3c073
42008b02:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008b06:	3c075637          	lui	a2,0x3c075
42008b0a:	ec060613          	add	a2,a2,-320 # 3c074ec0 <_flash_rodata_start+0x4da0>
42008b0e:	0a458593          	add	a1,a1,164
42008b12:	450d                	li	a0,3
42008b14:	fe38b097          	auipc	ra,0xfe38b
42008b18:	ada080e7          	jalr	-1318(ra) # 403935ee <esp_log_write>
# 		conn_handle = BLE_HS_CONN_HANDLE_NONE;
42008b1c:	577d                	li	a4,-1
42008b1e:	84e19723          	sh	a4,-1970(gp) # 3fc9524e <conn_handle>
# 		subscribed = false;
42008b22:	3fc997b7          	lui	a5,0x3fc99
42008b26:	36078a23          	sb	zero,884(a5) # 3fc99374 <subscribed>
# 		mtu = BLE_ATT_MTU_DFLT;
42008b2a:	475d                	li	a4,23
42008b2c:	84e19623          	sh	a4,-1972(gp) # 3fc9524c <mtu>
# 		advertise();
42008b30:	dfdff0ef          	jal	4200892c <advertise>
# }
42008b34:	4501                	li	a0,0
42008b36:	40b2                	lw	ra,12(sp)
42008b38:	4422                	lw	s0,8(sp)
42008b3a:	0141                	add	sp,sp,16
42008b3c:	8082                	ret
# 		if (event->connect.status == 0) {
42008b3e:	415c                	lw	a5,4(a0)
42008b40:	e3a9                	bnez	a5,42008b82 <gap_event+0xc4>
# 			conn_handle = event->connect.conn_handle;
42008b42:	00855703          	lhu	a4,8(a0)
42008b46:	84e19723          	sh	a4,-1970(gp) # 3fc9524e <conn_handle>
# 			stats.connects++;
42008b4a:	3fc997b7          	lui	a5,0x3fc99
42008b4e:	abc78793          	add	a5,a5,-1348 # 3fc98abc <stats>
42008b52:	4398                	lw	a4,0(a5)
42008b54:	0705                	add	a4,a4,1
42008b56:	c398                	sw	a4,0(a5)
# 			ESP_LOGI(TAG, "client connected");
42008b58:	fe38b097          	auipc	ra,0xfe38b
42008b5c:	b84080e7          	jalr	-1148(ra) # 403936dc <esp_log_timestamp>
42008b60:	86aa                	mv	a3,a0
42008b62:	3c0735b7          	lui	a1,0x3c073
42008b66:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008b6a:	3c075637          	lui	a2,0x3c075
42008b6e:	e9460613          	add	a2,a2,-364 # 3c074e94 <_flash_rodata_start+0x4d74>
42008b72:	0a458593          	add	a1,a1,164
42008b76:	450d                	li	a0,3
42008b78:	fe38b097          	auipc	ra,0xfe38b
42008b7c:	a76080e7          	jalr	-1418(ra) # 403935ee <esp_log_write>
42008b80:	bf55                	j	42008b34 <gap_event+0x76>
# 			advertise();
42008b82:	dabff0ef          	jal	4200892c <advertise>
42008b86:	b77d                	j	42008b34 <gap_event+0x76>
# 		if (event->subscribe.attr_handle == packets_handle) {
42008b88:	00655703          	lhu	a4,6(a0)
42008b8c:	3fc997b7          	lui	a5,0x3fc99
42008b90:	3787d783          	lhu	a5,888(a5) # 3fc99378 <packets_handle>
42008b94:	faf710e3          	bne	a4,a5,42008b34 <gap_event+0x76>
# 			subscribed = event->subscribe.cur_notify;
42008b98:	451c                	lw	a5,8(a0)
42008b9a:	83a5                	srl	a5,a5,0x9
42008b9c:	8b85                	and	a5,a5,1
42008b9e:	3fc99437          	lui	s0,0x3fc99
42008ba2:	36f40a23          	sb	a5,884(s0) # 3fc99374 <subscribed>
# 			ESP_LOGI(TAG, "notifications %s", subscribed ? "on" : "off");
42008ba6:	fe38b097          	auipc	ra,0xfe38b
42008baa:	b36080e7          	jalr	-1226(ra) # 403936dc <esp_log_timestamp>
42008bae:	86aa                	mv	a3,a0
42008bb0:	37444783          	lbu	a5,884(s0)
42008bb4:	0ff7f793          	zext.b	a5,a5
42008bb8:	c78d                	beqz	a5,42008be2 <gap_event+0x124>
42008bba:	3c0717b7          	lui	a5,0x3c071
42008bbe:	d4c78793          	add	a5,a5,-692 # 3c070d4c <_flash_rodata_start+0xc2c>
42008bc2:	3c0735b7          	lui	a1,0x3c073
42008bc6:	0a458713          	add	a4,a1,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008bca:	3c075637          	lui	a2,0x3c075
42008bce:	efc60613          	add	a2,a2,-260 # 3c074efc <_flash_rodata_start+0x4ddc>
42008bd2:	0a458593          	add	a1,a1,164
42008bd6:	450d                	li	a0,3
42008bd8:	fe38b097          	auipc	ra,0xfe38b
42008bdc:	a16080e7          	jalr	-1514(ra) # 403935ee <esp_log_write>
42008be0:	bf91                	j	42008b34 <gap_event+0x76>
42008be2:	3c0747b7          	lui	a5,0x3c074
42008be6:	7fc78793          	add	a5,a5,2044 # 3c0747fc <_flash_rodata_start+0x46dc>
42008bea:	bfe1                	j	42008bc2 <gap_event+0x104>
# 		advertise();
42008bec:	d41ff0ef          	jal	4200892c <advertise>
# 		break;
42008bf0:	b791                	j	42008b34 <gap_event+0x76>

42008bf2 <flush>:
# static uint8_t batch[NOTIFY_MAX];
# static size_t batch_len;
# static unsigned batch_count;
# 
# static void flush(void)
# {
42008bf2:	1141                	add	sp,sp,-16
42008bf4:	c606                	sw	ra,12(sp)
# 	if (batch_len == 0) {
42008bf6:	3fc997b7          	lui	a5,0x3fc99
42008bfa:	3707a583          	lw	a1,880(a5) # 3fc99370 <batch_len>
42008bfe:	e581                	bnez	a1,42008c06 <flush+0x14>
# 		stats.dropped += batch_count;
# 	}
# 
# 	batch_len = 0;
# 	batch_count = 0;
# }
42008c00:	40b2                	lw	ra,12(sp)
42008c02:	0141                	add	sp,sp,16
42008c04:	8082                	ret
# 	struct os_mbuf *om = ble_hs_mbuf_from_flat(batch, batch_len);
42008c06:	05c2                	sll	a1,a1,0x10
42008c08:	81c1                	srl	a1,a1,0x10
42008c0a:	3fc99537          	lui	a0,0x3fc99
42008c0e:	9c850513          	add	a0,a0,-1592 # 3fc989c8 <batch>
42008c12:	104070ef          	jal	4200fd16 <ble_hs_mbuf_from_flat>
42008c16:	862a                	mv	a2,a0
# 		    : BLE_HS_ENOMEM;
42008c18:	c90d                	beqz	a0,42008c4a <flush+0x58>
# 	int rc = om ? ble_gatts_notify_custom(conn_handle, packets_handle, om)
42008c1a:	84e1d503          	lhu	a0,-1970(gp) # 3fc9524e <conn_handle>
42008c1e:	3fc997b7          	lui	a5,0x3fc99
42008c22:	3787d583          	lhu	a1,888(a5) # 3fc99378 <packets_handle>
42008c26:	4b1060ef          	jal	4200f8d6 <ble_gatts_notify_custom>
# 	if (rc == 0) {
42008c2a:	e105                	bnez	a0,42008c4a <flush+0x58>
# 		stats.notifications++;
42008c2c:	3fc997b7          	lui	a5,0x3fc99
42008c30:	abc78793          	add	a5,a5,-1348 # 3fc98abc <stats>
42008c34:	43d8                	lw	a4,4(a5)
42008c36:	0705                	add	a4,a4,1
42008c38:	c3d8                	sw	a4,4(a5)
# 		stats.packets += batch_count;
42008c3a:	4798                	lw	a4,8(a5)
42008c3c:	3fc996b7          	lui	a3,0x3fc99
42008c40:	36c6a683          	lw	a3,876(a3) # 3fc9936c <batch_count>
42008c44:	9736                	add	a4,a4,a3
42008c46:	c798                	sw	a4,8(a5)
42008c48:	a821                	j	42008c60 <flush+0x6e>
# 		stats.dropped += batch_count;
42008c4a:	3fc997b7          	lui	a5,0x3fc99
42008c4e:	abc78793          	add	a5,a5,-1348 # 3fc98abc <stats>
42008c52:	47d8                	lw	a4,12(a5)
42008c54:	3fc996b7          	lui	a3,0x3fc99
42008c58:	36c6a683          	lw	a3,876(a3) # 3fc9936c <batch_count>
42008c5c:	9736                	add	a4,a4,a3
42008c5e:	c7d8                	sw	a4,12(a5)
# 	batch_len = 0;
42008c60:	3fc997b7          	lui	a5,0x3fc99
42008c64:	3607a823          	sw	zero,880(a5) # 3fc99370 <batch_len>
# 	batch_count = 0;
42008c68:	3fc997b7          	lui	a5,0x3fc99
42008c6c:	3607a623          	sw	zero,876(a5) # 3fc9936c <batch_count>
42008c70:	bf41                	j	42008c00 <flush+0xe>

42008c72 <cmd_ble>:
# }
# 
# /* ---- CLI ---- */
# 
# static int cmd_ble(int argc, char **argv)
# {
42008c72:	1141                	add	sp,sp,-16
42008c74:	c606                	sw	ra,12(sp)
42008c76:	c422                	sw	s0,8(sp)
# 	bool connected = conn_handle != BLE_HS_CONN_HANDLE_NONE;
42008c78:	84e1d403          	lhu	s0,-1970(gp) # 3fc9524e <conn_handle>
42008c7c:	0442                	sll	s0,s0,0x10
42008c7e:	8041                	srl	s0,s0,0x10
# 
# 	printf("name           %s\n", BLE_DEVICE_NAME);
42008c80:	3c0755b7          	lui	a1,0x3c075
42008c84:	e0058593          	add	a1,a1,-512 # 3c074e00 <_flash_rodata_start+0x4ce0>
42008c88:	3c075537          	lui	a0,0x3c075
42008c8c:	f5c50513          	add	a0,a0,-164 # 3c074f5c <_flash_rodata_start+0x4e3c>
42008c90:	272520ef          	jal	4205af02 <printf>
# 	printf("state          %s\n",
42008c94:	67c1                	lui	a5,0x10
42008c96:	17fd                	add	a5,a5,-1 # ffff <_rtc_fast_length+0xfdff>
42008c98:	00f40e63          	beq	s0,a5,42008cb4 <cmd_ble+0x42>
# 	       !connected ? "advertising" : subscribed ? "streaming" : "connected (not subscribed)");
42008c9c:	3fc997b7          	lui	a5,0x3fc99
42008ca0:	3747c783          	lbu	a5,884(a5) # 3fc99374 <subscribed>
42008ca4:	0ff7f793          	zext.b	a5,a5
42008ca8:	cfad                	beqz	a5,42008d22 <cmd_ble+0xb0>
42008caa:	3c0755b7          	lui	a1,0x3c075
42008cae:	f5058593          	add	a1,a1,-176 # 3c074f50 <_flash_rodata_start+0x4e30>
42008cb2:	a029                	j	42008cbc <cmd_ble+0x4a>
# 	printf("state          %s\n",
42008cb4:	3c0755b7          	lui	a1,0x3c075
42008cb8:	f2858593          	add	a1,a1,-216 # 3c074f28 <_flash_rodata_start+0x4e08>
42008cbc:	3c075537          	lui	a0,0x3c075
42008cc0:	f7050513          	add	a0,a0,-144 # 3c074f70 <_flash_rodata_start+0x4e50>
42008cc4:	23e520ef          	jal	4205af02 <printf>
# 	printf("mtu            %u\n", (unsigned)mtu);
42008cc8:	84c1d583          	lhu	a1,-1972(gp) # 3fc9524c <mtu>
42008ccc:	3c075537          	lui	a0,0x3c075
42008cd0:	f8450513          	add	a0,a0,-124 # 3c074f84 <_flash_rodata_start+0x4e64>
42008cd4:	22e520ef          	jal	4205af02 <printf>
# 	printf("connects       %" PRIu32 "\n", stats.connects);
42008cd8:	3fc99437          	lui	s0,0x3fc99
42008cdc:	abc40413          	add	s0,s0,-1348 # 3fc98abc <stats>
42008ce0:	400c                	lw	a1,0(s0)
42008ce2:	3c075537          	lui	a0,0x3c075
42008ce6:	f9850513          	add	a0,a0,-104 # 3c074f98 <_flash_rodata_start+0x4e78>
42008cea:	218520ef          	jal	4205af02 <printf>
# 	printf("notifications  %" PRIu32 "\n", stats.notifications);
42008cee:	404c                	lw	a1,4(s0)
42008cf0:	3c075537          	lui	a0,0x3c075
42008cf4:	fac50513          	add	a0,a0,-84 # 3c074fac <_flash_rodata_start+0x4e8c>
42008cf8:	20a520ef          	jal	4205af02 <printf>
# 	printf("packets sent   %" PRIu32 "\n", stats.packets);
42008cfc:	440c                	lw	a1,8(s0)
42008cfe:	3c075537          	lui	a0,0x3c075
42008d02:	fc050513          	add	a0,a0,-64 # 3c074fc0 <_flash_rodata_start+0x4ea0>
42008d06:	1fc520ef          	jal	4205af02 <printf>
# 	printf("packets lost   %" PRIu32 "\n", stats.dropped);
42008d0a:	444c                	lw	a1,12(s0)
42008d0c:	3c075537          	lui	a0,0x3c075
42008d10:	fd450513          	add	a0,a0,-44 # 3c074fd4 <_flash_rodata_start+0x4eb4>
42008d14:	1ee520ef          	jal	4205af02 <printf>
# 	return 0;
# }
42008d18:	4501                	li	a0,0
42008d1a:	40b2                	lw	ra,12(sp)
42008d1c:	4422                	lw	s0,8(sp)
42008d1e:	0141                	add	sp,sp,16
42008d20:	8082                	ret
# 	       !connected ? "advertising" : subscribed ? "streaming" : "connected (not subscribed)");
42008d22:	3c0755b7          	lui	a1,0x3c075
42008d26:	f3458593          	add	a1,a1,-204 # 3c074f34 <_flash_rodata_start+0x4e14>
42008d2a:	bf49                	j	42008cbc <cmd_ble+0x4a>

42008d2c <ble_link_init>:
# {
42008d2c:	1141                	add	sp,sp,-16
42008d2e:	c606                	sw	ra,12(sp)
# 	esp_err_t err = nvs_flash_init();
42008d30:	5851e0ef          	jal	42027ab4 <nvs_flash_init>
# 	if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
42008d34:	6785                	lui	a5,0x1
42008d36:	10d78793          	add	a5,a5,269 # 110d <_rtc_fast_length+0xf0d>
42008d3a:	08f50063          	beq	a0,a5,42008dba <ble_link_init+0x8e>
42008d3e:	6785                	lui	a5,0x1
42008d40:	11078793          	add	a5,a5,272 # 1110 <_rtc_fast_length+0xf10>
42008d44:	06f50b63          	beq	a0,a5,42008dba <ble_link_init+0x8e>
# 	ESP_ERROR_CHECK(err);
42008d48:	e14d                	bnez	a0,42008dea <ble_link_init+0xbe>
# 	ESP_ERROR_CHECK(nimble_port_init());
42008d4a:	0320d0ef          	jal	42015d7c <nimble_port_init>
42008d4e:	e161                	bnez	a0,42008e0e <ble_link_init+0xe2>
# 	ble_hs_cfg.sync_cb = on_sync;
42008d50:	3fc967b7          	lui	a5,0x3fc96
42008d54:	76078793          	add	a5,a5,1888 # 3fc96760 <ble_hs_cfg>
42008d58:	42009737          	lui	a4,0x42009
42008d5c:	a6c70713          	add	a4,a4,-1428 # 42008a6c <on_sync>
42008d60:	cb98                	sw	a4,16(a5)
# 	ble_hs_cfg.reset_cb = on_reset;
42008d62:	42009737          	lui	a4,0x42009
42008d66:	8f270713          	add	a4,a4,-1806 # 420088f2 <on_reset>
42008d6a:	c7d8                	sw	a4,12(a5)
# 	ble_svc_gap_init();
42008d6c:	200040ef          	jal	4200cf6c <ble_svc_gap_init>
# 	ble_svc_gatt_init();
42008d70:	7a1030ef          	jal	4200cd10 <ble_svc_gatt_init>
# 	ESP_ERROR_CHECK(ble_gatts_count_cfg(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
42008d74:	3c07e537          	lui	a0,0x3c07e
42008d78:	be850513          	add	a0,a0,-1048 # 3c07dbe8 <gatt_svcs>
42008d7c:	0820c0ef          	jal	42014dfe <ble_gatts_count_cfg>
42008d80:	e94d                	bnez	a0,42008e32 <ble_link_init+0x106>
# 	ESP_ERROR_CHECK(ble_gatts_add_svcs(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
42008d82:	3c07e537          	lui	a0,0x3c07e
42008d86:	be850513          	add	a0,a0,-1048 # 3c07dbe8 <gatt_svcs>
42008d8a:	0080c0ef          	jal	42014d92 <ble_gatts_add_svcs>
42008d8e:	e569                	bnez	a0,42008e58 <ble_link_init+0x12c>
# 	ble_svc_gap_device_name_set(BLE_DEVICE_NAME);
42008d90:	3c075537          	lui	a0,0x3c075
42008d94:	e0050513          	add	a0,a0,-512 # 3c074e00 <_flash_rodata_start+0x4ce0>
42008d98:	188040ef          	jal	4200cf20 <ble_svc_gap_device_name_set>
# 	leds_set_change_cb(on_leds_changed);
42008d9c:	42008537          	lui	a0,0x42008
42008da0:	79c50513          	add	a0,a0,1948 # 4200879c <on_leds_changed>
42008da4:	7a5000ef          	jal	42009d48 <leds_set_change_cb>
# 	nimble_port_freertos_init(host_task);
42008da8:	42008537          	lui	a0,0x42008
42008dac:	78650513          	add	a0,a0,1926 # 42008786 <host_task>
42008db0:	1860d0ef          	jal	42015f36 <nimble_port_freertos_init>
# }
42008db4:	40b2                	lw	ra,12(sp)
42008db6:	0141                	add	sp,sp,16
42008db8:	8082                	ret
# 		ESP_ERROR_CHECK(nvs_flash_erase());
42008dba:	6711e0ef          	jal	42027c2a <nvs_flash_erase>
42008dbe:	e501                	bnez	a0,42008dc6 <ble_link_init+0x9a>
# 		err = nvs_flash_init();
42008dc0:	4f51e0ef          	jal	42027ab4 <nvs_flash_init>
42008dc4:	b751                	j	42008d48 <ble_link_init+0x1c>
# 		ESP_ERROR_CHECK(nvs_flash_erase());
42008dc6:	3c075737          	lui	a4,0x3c075
42008dca:	fe870713          	add	a4,a4,-24 # 3c074fe8 <_flash_rodata_start+0x4ec8>
42008dce:	3c07e6b7          	lui	a3,0x3c07e
42008dd2:	bd868693          	add	a3,a3,-1064 # 3c07dbd8 <__func__.1>
42008dd6:	10a00613          	li	a2,266
42008dda:	3c0755b7          	lui	a1,0x3c075
42008dde:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42008de2:	fe384097          	auipc	ra,0xfe384
42008de6:	c0c080e7          	jalr	-1012(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(err);
42008dea:	3c075737          	lui	a4,0x3c075
42008dee:	01070713          	add	a4,a4,16 # 3c075010 <_flash_rodata_start+0x4ef0>
42008df2:	3c07e6b7          	lui	a3,0x3c07e
42008df6:	bd868693          	add	a3,a3,-1064 # 3c07dbd8 <__func__.1>
42008dfa:	10d00613          	li	a2,269
42008dfe:	3c0755b7          	lui	a1,0x3c075
42008e02:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42008e06:	fe384097          	auipc	ra,0xfe384
42008e0a:	be8080e7          	jalr	-1048(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(nimble_port_init());
42008e0e:	3c075737          	lui	a4,0x3c075
42008e12:	01470713          	add	a4,a4,20 # 3c075014 <_flash_rodata_start+0x4ef4>
42008e16:	3c07e6b7          	lui	a3,0x3c07e
42008e1a:	bd868693          	add	a3,a3,-1064 # 3c07dbd8 <__func__.1>
42008e1e:	10f00613          	li	a2,271
42008e22:	3c0755b7          	lui	a1,0x3c075
42008e26:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42008e2a:	fe384097          	auipc	ra,0xfe384
42008e2e:	bc4080e7          	jalr	-1084(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(ble_gatts_count_cfg(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
42008e32:	3c075737          	lui	a4,0x3c075
42008e36:	02870713          	add	a4,a4,40 # 3c075028 <_flash_rodata_start+0x4f08>
42008e3a:	3c07e6b7          	lui	a3,0x3c07e
42008e3e:	bd868693          	add	a3,a3,-1064 # 3c07dbd8 <__func__.1>
42008e42:	11500613          	li	a2,277
42008e46:	3c0755b7          	lui	a1,0x3c075
42008e4a:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42008e4e:	557d                	li	a0,-1
42008e50:	fe384097          	auipc	ra,0xfe384
42008e54:	b9e080e7          	jalr	-1122(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(ble_gatts_add_svcs(gatt_svcs) == 0 ? ESP_OK : ESP_FAIL);
42008e58:	3c075737          	lui	a4,0x3c075
42008e5c:	06070713          	add	a4,a4,96 # 3c075060 <_flash_rodata_start+0x4f40>
42008e60:	3c07e6b7          	lui	a3,0x3c07e
42008e64:	bd868693          	add	a3,a3,-1064 # 3c07dbd8 <__func__.1>
42008e68:	11600613          	li	a2,278
42008e6c:	3c0755b7          	lui	a1,0x3c075
42008e70:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42008e74:	557d                	li	a0,-1
42008e76:	fe384097          	auipc	ra,0xfe384
42008e7a:	b78080e7          	jalr	-1160(ra) # 4038c9ee <_esp_error_check_failed>

42008e7e <ble_link_tx_task>:
# {
42008e7e:	7139                	add	sp,sp,-64
42008e80:	de06                	sw	ra,60(sp)
42008e82:	dc22                	sw	s0,56(sp)
42008e84:	da26                	sw	s1,52(sp)
42008e86:	d84a                	sw	s2,48(sp)
42008e88:	d64e                	sw	s3,44(sp)
42008e8a:	d452                	sw	s4,40(sp)
42008e8c:	d256                	sw	s5,36(sp)
# 	TickType_t batch_start = 0;
42008e8e:	4481                	li	s1,0
# 	bool was_subscribed = false;
42008e90:	4901                	li	s2,0
42008e92:	a0f9                	j	42008f60 <ble_link_tx_task+0xe2>
# 		if (!was_subscribed) {
42008e94:	0e090c63          	beqz	s2,42008f8c <ble_link_tx_task+0x10e>
# 		size_t cap = mtu - 3;
42008e98:	84c1d403          	lhu	s0,-1972(gp) # 3fc9524c <mtu>
42008e9c:	1475                	add	s0,s0,-3
# 		if (cap > sizeof(batch)) {
42008e9e:	0f400793          	li	a5,244
42008ea2:	0087f463          	bgeu	a5,s0,42008eaa <ble_link_tx_task+0x2c>
# 			cap = sizeof(batch);
42008ea6:	0f400413          	li	s0,244
# 		if (dcc_receive(&p, pdMS_TO_TICKS(BLE_BATCH_MS))) {
42008eaa:	4595                	li	a1,5
42008eac:	0028                	add	a0,sp,8
42008eae:	c92ff0ef          	jal	42008340 <dcc_receive>
42008eb2:	c549                	beqz	a0,42008f3c <ble_link_tx_task+0xbe>
# 			size_t rec = RECORD_HEADER + p.len;
42008eb4:	01014a03          	lbu	s4,16(sp)
42008eb8:	0a15                	add	s4,s4,5
# 			if (batch_len + rec > cap) {
42008eba:	3fc997b7          	lui	a5,0x3fc99
42008ebe:	3707a783          	lw	a5,880(a5) # 3fc99370 <batch_len>
42008ec2:	97d2                	add	a5,a5,s4
42008ec4:	0cf46b63          	bltu	s0,a5,42008f9a <ble_link_tx_task+0x11c>
# 			if (batch_len == 0) {
42008ec8:	3fc997b7          	lui	a5,0x3fc99
42008ecc:	3707a783          	lw	a5,880(a5) # 3fc99370 <batch_len>
42008ed0:	cbe1                	beqz	a5,42008fa0 <ble_link_tx_task+0x122>
# 			uint32_t t_ms = (uint32_t)(p.time_us / 1000);
42008ed2:	3e800613          	li	a2,1000
42008ed6:	4681                	li	a3,0
42008ed8:	4522                	lw	a0,8(sp)
42008eda:	45b2                	lw	a1,12(sp)
42008edc:	fdff8097          	auipc	ra,0xfdff8
42008ee0:	8d8080e7          	jalr	-1832(ra) # 400007b4 <__divdi3>
# 			uint8_t *r = batch + batch_len;
42008ee4:	3fc99ab7          	lui	s5,0x3fc99
42008ee8:	370aa983          	lw	s3,880(s5) # 3fc99370 <batch_len>
42008eec:	3fc997b7          	lui	a5,0x3fc99
42008ef0:	9c878793          	add	a5,a5,-1592 # 3fc989c8 <batch>
42008ef4:	97ce                	add	a5,a5,s3
# 			r[0] = p.len;
42008ef6:	01014603          	lbu	a2,16(sp)
42008efa:	00c78023          	sb	a2,0(a5)
# 			r[1] = (uint8_t)t_ms;
42008efe:	00a780a3          	sb	a0,1(a5)
# 			r[2] = (uint8_t)(t_ms >> 8);
42008f02:	00855713          	srl	a4,a0,0x8
42008f06:	00e78123          	sb	a4,2(a5)
# 			r[3] = (uint8_t)(t_ms >> 16);
42008f0a:	01055713          	srl	a4,a0,0x10
42008f0e:	00e781a3          	sb	a4,3(a5)
# 			r[4] = (uint8_t)(t_ms >> 24);
42008f12:	8161                	srl	a0,a0,0x18
42008f14:	00a78223          	sb	a0,4(a5)
# 			memcpy(r + RECORD_HEADER, p.data, p.len);
42008f18:	01110593          	add	a1,sp,17
42008f1c:	00578513          	add	a0,a5,5
42008f20:	fdff7097          	auipc	ra,0xfdff7
42008f24:	438080e7          	jalr	1080(ra) # 40000358 <memcpy>
# 			batch_len += rec;
42008f28:	99d2                	add	s3,s3,s4
42008f2a:	373aa823          	sw	s3,880(s5)
# 			batch_count++;
42008f2e:	3fc99737          	lui	a4,0x3fc99
42008f32:	36c72783          	lw	a5,876(a4) # 3fc9936c <batch_count>
42008f36:	0785                	add	a5,a5,1
42008f38:	36f72623          	sw	a5,876(a4)
# 		if (batch_len > 0 &&
42008f3c:	3fc997b7          	lui	a5,0x3fc99
42008f40:	3707a783          	lw	a5,880(a5) # 3fc99370 <batch_len>
42008f44:	cf91                	beqz	a5,42008f60 <ble_link_tx_task+0xe2>
# 		    (batch_len + RECORD_MAX > cap ||
42008f46:	07b5                	add	a5,a5,13
# 		if (batch_len > 0 &&
42008f48:	00f46a63          	bltu	s0,a5,42008f5c <ble_link_tx_task+0xde>
# 		     xTaskGetTickCount() - batch_start >= pdMS_TO_TICKS(BLE_BATCH_MS))) {
42008f4c:	fe386097          	auipc	ra,0xfe386
42008f50:	8e4080e7          	jalr	-1820(ra) # 4038e830 <xTaskGetTickCount>
42008f54:	8d05                	sub	a0,a0,s1
# 		    (batch_len + RECORD_MAX > cap ||
42008f56:	4791                	li	a5,4
42008f58:	00a7f463          	bgeu	a5,a0,42008f60 <ble_link_tx_task+0xe2>
# 			flush();
42008f5c:	c97ff0ef          	jal	42008bf2 <flush>
# 		if (!subscribed) {
42008f60:	3fc997b7          	lui	a5,0x3fc99
42008f64:	3747c403          	lbu	s0,884(a5) # 3fc99374 <subscribed>
42008f68:	0ff47413          	zext.b	s0,s0
42008f6c:	f405                	bnez	s0,42008e94 <ble_link_tx_task+0x16>
# 			batch_len = 0;
42008f6e:	3fc997b7          	lui	a5,0x3fc99
42008f72:	3607a823          	sw	zero,880(a5) # 3fc99370 <batch_len>
# 			batch_count = 0;
42008f76:	3fc997b7          	lui	a5,0x3fc99
42008f7a:	3607a623          	sw	zero,876(a5) # 3fc9936c <batch_count>
# 			vTaskDelay(pdMS_TO_TICKS(100));
42008f7e:	4529                	li	a0,10
42008f80:	fe386097          	auipc	ra,0xfe386
42008f84:	c5e080e7          	jalr	-930(ra) # 4038ebde <vTaskDelay>
# 			was_subscribed = false;
42008f88:	8922                	mv	s2,s0
42008f8a:	bfd9                	j	42008f60 <ble_link_tx_task+0xe2>
# 			while (dcc_receive(&stale, 0)) {
42008f8c:	4581                	li	a1,0
42008f8e:	0028                	add	a0,sp,8
42008f90:	bb0ff0ef          	jal	42008340 <dcc_receive>
42008f94:	fd65                	bnez	a0,42008f8c <ble_link_tx_task+0x10e>
# 			was_subscribed = true;
42008f96:	8922                	mv	s2,s0
42008f98:	b701                	j	42008e98 <ble_link_tx_task+0x1a>
# 				flush();
42008f9a:	c59ff0ef          	jal	42008bf2 <flush>
42008f9e:	b72d                	j	42008ec8 <ble_link_tx_task+0x4a>
# 				batch_start = xTaskGetTickCount();
42008fa0:	fe386097          	auipc	ra,0xfe386
42008fa4:	890080e7          	jalr	-1904(ra) # 4038e830 <xTaskGetTickCount>
42008fa8:	84aa                	mv	s1,a0
42008faa:	b725                	j	42008ed2 <ble_link_tx_task+0x54>

42008fac <ble_link_register_cli_commands>:
# 
# void ble_link_register_cli_commands(void)
# {
42008fac:	7179                	add	sp,sp,-48
42008fae:	d606                	sw	ra,44(sp)
# 	const esp_console_cmd_t ble_cmd = {
42008fb0:	c202                	sw	zero,4(sp)
42008fb2:	c402                	sw	zero,8(sp)
42008fb4:	c602                	sw	zero,12(sp)
42008fb6:	c802                	sw	zero,16(sp)
42008fb8:	ca02                	sw	zero,20(sp)
42008fba:	cc02                	sw	zero,24(sp)
42008fbc:	ce02                	sw	zero,28(sp)
42008fbe:	3c0737b7          	lui	a5,0x3c073
42008fc2:	0a478793          	add	a5,a5,164 # 3c0730a4 <_flash_rodata_start+0x2f84>
42008fc6:	c23e                	sw	a5,4(sp)
42008fc8:	3c0757b7          	lui	a5,0x3c075
42008fcc:	09878793          	add	a5,a5,152 # 3c075098 <_flash_rodata_start+0x4f78>
42008fd0:	c43e                	sw	a5,8(sp)
42008fd2:	420097b7          	lui	a5,0x42009
42008fd6:	c7278793          	add	a5,a5,-910 # 42008c72 <cmd_ble>
42008fda:	c83e                	sw	a5,16(sp)
# 		.command = "ble",
# 		.help = "BLE link state and counters (streaming to the web page)",
# 		.func = &cmd_ble,
# 	};
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&ble_cmd));
42008fdc:	0048                	add	a0,sp,4
42008fde:	7d9000ef          	jal	42009fb6 <esp_console_cmd_register>
42008fe2:	e501                	bnez	a0,42008fea <ble_link_register_cli_commands+0x3e>
# }
42008fe4:	50b2                	lw	ra,44(sp)
42008fe6:	6145                	add	sp,sp,48
42008fe8:	8082                	ret
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&ble_cmd));
42008fea:	3c075737          	lui	a4,0x3c075
42008fee:	0d070713          	add	a4,a4,208 # 3c0750d0 <_flash_rodata_start+0x4fb0>
42008ff2:	3c07e6b7          	lui	a3,0x3c07e
42008ff6:	bb868693          	add	a3,a3,-1096 # 3c07dbb8 <__func__.0>
42008ffa:	18d00613          	li	a2,397
42008ffe:	3c0755b7          	lui	a1,0x3c075
42009002:	ffc58593          	add	a1,a1,-4 # 3c074ffc <_flash_rodata_start+0x4edc>
42009006:	fe384097          	auipc	ra,0xfe384
4200900a:	9e8080e7          	jalr	-1560(ra) # 4038c9ee <_esp_error_check_failed>

