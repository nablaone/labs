# main.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

420077b2 <app_main>:
#  * matching, outputs -- see ../docs/dcc-accessory-primer.md) slot in here
#  * the same way: an _init() and _register_cli_commands() call, plus an
#  * xTaskCreate() if they need one.
#  */
# void app_main(void)
# {
420077b2:	1141                	add	sp,sp,-16
420077b4:	c606                	sw	ra,12(sp)
420077b6:	c422                	sw	s0,8(sp)
# 	console_init();
420077b8:	24f1                	jal	42007a84 <console_init>
# 
# 	heartbeat_task_init();
420077ba:	28e9                	jal	42007894 <heartbeat_task_init>
# 	dcc_init();
420077bc:	6b9000ef          	jal	42008674 <dcc_init>
# 	dcc_register_cli_commands();
420077c0:	653000ef          	jal	42008612 <dcc_register_cli_commands>
# 	leds_init();
420077c4:	679010ef          	jal	4200963c <leds_init>
# 	leds_register_cli_commands();
420077c8:	58a020ef          	jal	42009d52 <leds_register_cli_commands>
# 	ble_link_init();
420077cc:	560010ef          	jal	42008d2c <ble_link_init>
# 	ble_link_register_cli_commands();
420077d0:	7dc010ef          	jal	42008fac <ble_link_register_cli_commands>
# 
#         /*
#          * Call the "PinnedToCore" version with tskNO_AFFINITY to create
#          * an unpinned task.
#          */
#         return xTaskCreatePinnedToCore( pxTaskCode,
420077d4:	80000437          	lui	s0,0x80000
420077d8:	fff40813          	add	a6,s0,-1 # 7fffffff <SYSTEM+0x1ff3ffff>
420077dc:	4781                	li	a5,0
420077de:	4715                	li	a4,5
420077e0:	4681                	li	a3,0
420077e2:	6605                	lui	a2,0x1
420077e4:	80060613          	add	a2,a2,-2048 # 800 <_rtc_fast_length+0x600>
420077e8:	3c0745b7          	lui	a1,0x3c074
420077ec:	5d858593          	add	a1,a1,1496 # 3c0745d8 <_flash_rodata_start+0x44b8>
420077f0:	42008537          	lui	a0,0x42008
420077f4:	8b250513          	add	a0,a0,-1870 # 420078b2 <heartbeat_task>
420077f8:	fe388097          	auipc	ra,0xfe388
420077fc:	db4080e7          	jalr	-588(ra) # 4038f5ac <xTaskCreatePinnedToCore>
42007800:	fff40813          	add	a6,s0,-1
42007804:	4781                	li	a5,0
42007806:	4715                	li	a4,5
42007808:	4681                	li	a3,0
4200780a:	6605                	lui	a2,0x1
4200780c:	3c0745b7          	lui	a1,0x3c074
42007810:	5e458593          	add	a1,a1,1508 # 3c0745e4 <_flash_rodata_start+0x44c4>
42007814:	42009537          	lui	a0,0x42009
42007818:	e7e50513          	add	a0,a0,-386 # 42008e7e <ble_link_tx_task>
4200781c:	fe388097          	auipc	ra,0xfe388
42007820:	d90080e7          	jalr	-624(ra) # 4038f5ac <xTaskCreatePinnedToCore>
42007824:	fff40813          	add	a6,s0,-1
42007828:	4781                	li	a5,0
4200782a:	4715                	li	a4,5
4200782c:	4681                	li	a3,0
4200782e:	6605                	lui	a2,0x1
42007830:	3c0745b7          	lui	a1,0x3c074
42007834:	5ec58593          	add	a1,a1,1516 # 3c0745ec <_flash_rodata_start+0x44cc>
42007838:	42008537          	lui	a0,0x42008
4200783c:	b8250513          	add	a0,a0,-1150 # 42007b82 <console_task>
42007840:	fe388097          	auipc	ra,0xfe388
42007844:	d6c080e7          	jalr	-660(ra) # 4038f5ac <xTaskCreatePinnedToCore>
# 
# 	xTaskCreate(heartbeat_task, "heartbeat", 2048, NULL, 5, NULL);
# 	xTaskCreate(ble_link_tx_task, "ble_tx", 4096, NULL, 5, NULL);
# 	xTaskCreate(console_task, "console", 4096, NULL, 5, NULL);
# 
# 	ESP_LOGI(TAG, "dcc %s up -- press Enter for the CLI", FIRMWARE_VERSION);
42007848:	fe38c097          	auipc	ra,0xfe38c
4200784c:	e94080e7          	jalr	-364(ra) # 403936dc <esp_log_timestamp>
42007850:	86aa                	mv	a3,a0
42007852:	3c0747b7          	lui	a5,0x3c074
42007856:	5f478793          	add	a5,a5,1524 # 3c0745f4 <_flash_rodata_start+0x44d4>
4200785a:	3c0735b7          	lui	a1,0x3c073
4200785e:	41058713          	add	a4,a1,1040 # 3c073410 <_flash_rodata_start+0x32f0>
42007862:	3c074637          	lui	a2,0x3c074
42007866:	60860613          	add	a2,a2,1544 # 3c074608 <_flash_rodata_start+0x44e8>
4200786a:	41058593          	add	a1,a1,1040
4200786e:	450d                	li	a0,3
42007870:	fe38c097          	auipc	ra,0xfe38c
42007874:	d7e080e7          	jalr	-642(ra) # 403935ee <esp_log_write>
# }
42007878:	40b2                	lw	ra,12(sp)
4200787a:	4422                	lw	s0,8(sp)
4200787c:	0141                	add	sp,sp,16
4200787e:	8082                	ret

