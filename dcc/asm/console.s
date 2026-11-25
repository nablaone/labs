# console.c, disassembled from build/dcc.elf (ESP32-C3, rv32imc, -Og).
# Regenerate with: make asm
#
# Columns: address, instruction bytes (2 = compressed RVC, 4 = full), instruction.

420078d2 <cmd_exit>:
# 
# static volatile bool cli_exit_requested;
# 
# static int cmd_exit(int argc, char **argv)
# {
# 	cli_exit_requested = true;
420078d2:	3fc997b7          	lui	a5,0x3fc99
420078d6:	4705                	li	a4,1
420078d8:	34e78323          	sb	a4,838(a5) # 3fc99346 <cli_exit_requested>
# 	return 0;
# }
420078dc:	4501                	li	a0,0
420078de:	8082                	ret

420078e0 <register_core_cli_commands>:
# 
# static void register_core_cli_commands(void)
# {
420078e0:	715d                	add	sp,sp,-80
420078e2:	c686                	sw	ra,76(sp)
# 	esp_console_register_help_command();
420078e4:	0d5020ef          	jal	4200a1b8 <esp_console_register_help_command>
# 
# 	const esp_console_cmd_t version_cmd = {
420078e8:	d202                	sw	zero,36(sp)
420078ea:	d402                	sw	zero,40(sp)
420078ec:	d602                	sw	zero,44(sp)
420078ee:	d802                	sw	zero,48(sp)
420078f0:	da02                	sw	zero,52(sp)
420078f2:	dc02                	sw	zero,56(sp)
420078f4:	de02                	sw	zero,60(sp)
420078f6:	3c07a7b7          	lui	a5,0x3c07a
420078fa:	e0478793          	add	a5,a5,-508 # 3c079e04 <_flash_rodata_start+0x9ce4>
420078fe:	d23e                	sw	a5,36(sp)
42007900:	3c0747b7          	lui	a5,0x3c074
42007904:	64878793          	add	a5,a5,1608 # 3c074648 <_flash_rodata_start+0x4528>
42007908:	d43e                	sw	a5,40(sp)
4200790a:	420087b7          	lui	a5,0x42008
4200790e:	99e78793          	add	a5,a5,-1634 # 4200799e <cmd_version>
42007912:	d83e                	sw	a5,48(sp)
# 		.command = "version",
# 		.help = "Show firmware version",
# 		.func = &cmd_version,
# 	};
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&version_cmd));
42007914:	1048                	add	a0,sp,36
42007916:	6a0020ef          	jal	42009fb6 <esp_console_cmd_register>
4200791a:	ed15                	bnez	a0,42007956 <register_core_cli_commands+0x76>
# 
# 	const esp_console_cmd_t exit_cmd = {
4200791c:	c402                	sw	zero,8(sp)
4200791e:	c602                	sw	zero,12(sp)
42007920:	c802                	sw	zero,16(sp)
42007922:	ca02                	sw	zero,20(sp)
42007924:	cc02                	sw	zero,24(sp)
42007926:	ce02                	sw	zero,28(sp)
42007928:	d002                	sw	zero,32(sp)
4200792a:	3c0747b7          	lui	a5,0x3c074
4200792e:	69c78793          	add	a5,a5,1692 # 3c07469c <_flash_rodata_start+0x457c>
42007932:	c43e                	sw	a5,8(sp)
42007934:	3c0747b7          	lui	a5,0x3c074
42007938:	6a478793          	add	a5,a5,1700 # 3c0746a4 <_flash_rodata_start+0x4584>
4200793c:	c63e                	sw	a5,12(sp)
4200793e:	420087b7          	lui	a5,0x42008
42007942:	8d278793          	add	a5,a5,-1838 # 420078d2 <cmd_exit>
42007946:	ca3e                	sw	a5,20(sp)
# 		.command = "exit",
# 		.help = "Leave CLI mode, resume logging",
# 		.func = &cmd_exit,
# 	};
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&exit_cmd));
42007948:	0028                	add	a0,sp,8
4200794a:	66c020ef          	jal	42009fb6 <esp_console_cmd_register>
4200794e:	e515                	bnez	a0,4200797a <register_core_cli_commands+0x9a>
# }
42007950:	40b6                	lw	ra,76(sp)
42007952:	6161                	add	sp,sp,80
42007954:	8082                	ret
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&version_cmd));
42007956:	3c074737          	lui	a4,0x3c074
4200795a:	66070713          	add	a4,a4,1632 # 3c074660 <_flash_rodata_start+0x4540>
4200795e:	3c07e6b7          	lui	a3,0x3c07e
42007962:	a0c68693          	add	a3,a3,-1524 # 3c07da0c <__func__.0>
42007966:	02900613          	li	a2,41
4200796a:	3c0745b7          	lui	a1,0x3c074
4200796e:	68858593          	add	a1,a1,1672 # 3c074688 <_flash_rodata_start+0x4568>
42007972:	fe385097          	auipc	ra,0xfe385
42007976:	07c080e7          	jalr	124(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(esp_console_cmd_register(&exit_cmd));
4200797a:	3c074737          	lui	a4,0x3c074
4200797e:	6c470713          	add	a4,a4,1732 # 3c0746c4 <_flash_rodata_start+0x45a4>
42007982:	3c07e6b7          	lui	a3,0x3c07e
42007986:	a0c68693          	add	a3,a3,-1524 # 3c07da0c <__func__.0>
4200798a:	03000613          	li	a2,48
4200798e:	3c0745b7          	lui	a1,0x3c074
42007992:	68858593          	add	a1,a1,1672 # 3c074688 <_flash_rodata_start+0x4568>
42007996:	fe385097          	auipc	ra,0xfe385
4200799a:	058080e7          	jalr	88(ra) # 4038c9ee <_esp_error_check_failed>

4200799e <cmd_version>:
# {
4200799e:	1141                	add	sp,sp,-16
420079a0:	c606                	sw	ra,12(sp)
# 	printf("dcc %s (ESP-IDF %s)\n", FIRMWARE_VERSION, esp_get_idf_version());
420079a2:	203610ef          	jal	420693a4 <esp_get_idf_version>
420079a6:	862a                	mv	a2,a0
420079a8:	3c0745b7          	lui	a1,0x3c074
420079ac:	5f458593          	add	a1,a1,1524 # 3c0745f4 <_flash_rodata_start+0x44d4>
420079b0:	3c074537          	lui	a0,0x3c074
420079b4:	6e850513          	add	a0,a0,1768 # 3c0746e8 <_flash_rodata_start+0x45c8>
420079b8:	54a530ef          	jal	4205af02 <printf>
# }
420079bc:	4501                	li	a0,0
420079be:	40b2                	lw	ra,12(sp)
420079c0:	0141                	add	sp,sp,16
420079c2:	8082                	ret

420079c4 <run_cli>:
#  * Runs the read-eval-print loop until "exit". Logging is silenced for the
#  * duration so the prompt/output isn't interleaved with task log lines, and
#  * restored on the way out.
#  */
# static void run_cli(void)
# {
420079c4:	7179                	add	sp,sp,-48
420079c6:	d606                	sw	ra,44(sp)
420079c8:	d422                	sw	s0,40(sp)
# 	cli_exit_requested = false;
420079ca:	3fc997b7          	lui	a5,0x3fc99
420079ce:	34078323          	sb	zero,838(a5) # 3fc99346 <cli_exit_requested>
# 	esp_log_level_set("*", ESP_LOG_NONE);
420079d2:	4581                	li	a1,0
420079d4:	3c072537          	lui	a0,0x3c072
420079d8:	2c450513          	add	a0,a0,708 # 3c0722c4 <_flash_rodata_start+0x21a4>
420079dc:	13d620ef          	jal	4206a318 <esp_log_level_set>
# 	printf("\n-- CLI mode -- 'help' for commands, 'exit' to leave --\n");
420079e0:	3c074537          	lui	a0,0x3c074
420079e4:	70050513          	add	a0,a0,1792 # 3c074700 <_flash_rodata_start+0x45e0>
420079e8:	6ae530ef          	jal	4205b096 <puts>
# 
# 	while (!cli_exit_requested) {
420079ec:	a819                	j	42007a02 <run_cli+0x3e>
# 			linenoiseHistoryAdd(line);
# 
# 			int ret;
# 			esp_err_t err = esp_console_run(line, &ret);
# 			if (err == ESP_ERR_NOT_FOUND) {
# 				printf("Unknown command: '%s'\n", line);
420079ee:	85a2                	mv	a1,s0
420079f0:	3c074537          	lui	a0,0x3c074
420079f4:	74050513          	add	a0,a0,1856 # 3c074740 <_flash_rodata_start+0x4620>
420079f8:	50a530ef          	jal	4205af02 <printf>
# 			} else if (err != ESP_OK && err != ESP_ERR_INVALID_ARG) {
# 				printf("Command error: 0x%x\n", err);
# 			}
# 		}
# 
# 		linenoiseFree(line);
420079fc:	8522                	mv	a0,s0
420079fe:	64e030ef          	jal	4200b04c <linenoiseFree>
# 	while (!cli_exit_requested) {
42007a02:	3fc997b7          	lui	a5,0x3fc99
42007a06:	3467c783          	lbu	a5,838(a5) # 3fc99346 <cli_exit_requested>
42007a0a:	0ff7f793          	zext.b	a5,a5
42007a0e:	e3b9                	bnez	a5,42007a54 <run_cli+0x90>
# 		char *line = linenoise(CLI_PROMPT);
42007a10:	3c074537          	lui	a0,0x3c074
42007a14:	73850513          	add	a0,a0,1848 # 3c074738 <_flash_rodata_start+0x4618>
42007a18:	343030ef          	jal	4200b55a <linenoise>
42007a1c:	842a                	mv	s0,a0
# 		if (line == NULL) {
42007a1e:	c91d                	beqz	a0,42007a54 <run_cli+0x90>
# 		if (line[0] != '\0') {
42007a20:	00054783          	lbu	a5,0(a0)
42007a24:	dfe1                	beqz	a5,420079fc <run_cli+0x38>
# 			linenoiseHistoryAdd(line);
42007a26:	638030ef          	jal	4200b05e <linenoiseHistoryAdd>
# 			esp_err_t err = esp_console_run(line, &ret);
42007a2a:	006c                	add	a1,sp,12
42007a2c:	8522                	mv	a0,s0
42007a2e:	6be020ef          	jal	4200a0ec <esp_console_run>
# 			if (err == ESP_ERR_NOT_FOUND) {
42007a32:	10500793          	li	a5,261
42007a36:	faf50ce3          	beq	a0,a5,420079ee <run_cli+0x2a>
# 			} else if (err != ESP_OK && err != ESP_ERR_INVALID_ARG) {
42007a3a:	d169                	beqz	a0,420079fc <run_cli+0x38>
42007a3c:	10200793          	li	a5,258
42007a40:	faf50ee3          	beq	a0,a5,420079fc <run_cli+0x38>
# 				printf("Command error: 0x%x\n", err);
42007a44:	85aa                	mv	a1,a0
42007a46:	3c074537          	lui	a0,0x3c074
42007a4a:	75850513          	add	a0,a0,1880 # 3c074758 <_flash_rodata_start+0x4638>
42007a4e:	4b4530ef          	jal	4205af02 <printf>
42007a52:	b76d                	j	420079fc <run_cli+0x38>
# 	}
# 
# 	printf("-- leaving CLI mode --\n");
42007a54:	3c074537          	lui	a0,0x3c074
42007a58:	77050513          	add	a0,a0,1904 # 3c074770 <_flash_rodata_start+0x4650>
42007a5c:	63a530ef          	jal	4205b096 <puts>
# 	esp_log_level_set("*", ESP_LOG_INFO);
42007a60:	458d                	li	a1,3
42007a62:	3c072537          	lui	a0,0x3c072
42007a66:	2c450513          	add	a0,a0,708 # 3c0722c4 <_flash_rodata_start+0x21a4>
42007a6a:	0af620ef          	jal	4206a318 <esp_log_level_set>
# 	 * terminal that sends CRLF for Enter -- the VFS RX line-ending mode
# 	 * only special-cases '\r') so it doesn't immediately re-trigger
# 	 * console_task's "enter CLI mode" check below. The USB-Serial/JTAG
# 	 * driver has no flush call, so drain it with zero-timeout reads. */
# 	char discard[16];
# 	while (usb_serial_jtag_read_bytes(discard, sizeof(discard), 0) > 0) {
42007a6e:	4601                	li	a2,0
42007a70:	45c1                	li	a1,16
42007a72:	0808                	add	a0,sp,16
42007a74:	995fe0ef          	jal	42006408 <usb_serial_jtag_read_bytes>
42007a78:	fea04be3          	bgtz	a0,42007a6e <run_cli+0xaa>
# 	}
# }
42007a7c:	50b2                	lw	ra,44(sp)
42007a7e:	5422                	lw	s0,40(sp)
42007a80:	6145                	add	sp,sp,48
42007a82:	8082                	ret

42007a84 <console_init>:
# {
42007a84:	7179                	add	sp,sp,-48
42007a86:	d606                	sw	ra,44(sp)
# 	fflush(stdout);
42007a88:	fe388097          	auipc	ra,0xfe388
42007a8c:	e9e080e7          	jalr	-354(ra) # 4038f926 <__getreent>
42007a90:	4508                	lw	a0,8(a0)
42007a92:	1c9520ef          	jal	4205a45a <fflush>
# 	fsync(fileno(stdout));
42007a96:	fe388097          	auipc	ra,0xfe388
42007a9a:	e90080e7          	jalr	-368(ra) # 4038f926 <__getreent>
42007a9e:	4508                	lw	a0,8(a0)
42007aa0:	273520ef          	jal	4205a512 <fileno>
42007aa4:	f44ff0ef          	jal	420071e8 <esp_vfs_fsync>
# 	usb_serial_jtag_vfs_set_rx_line_endings(ESP_LINE_ENDINGS_CR);
42007aa8:	4505                	li	a0,1
42007aaa:	f4afe0ef          	jal	420061f4 <esp_vfs_dev_usb_serial_jtag_set_rx_line_endings>
# 	usb_serial_jtag_vfs_set_tx_line_endings(ESP_LINE_ENDINGS_CRLF);
42007aae:	4501                	li	a0,0
42007ab0:	f3cfe0ef          	jal	420061ec <esp_vfs_dev_usb_serial_jtag_set_tx_line_endings>
# 	usb_serial_jtag_driver_config_t usb_config = USB_SERIAL_JTAG_DRIVER_CONFIG_DEFAULT();
42007ab4:	10000793          	li	a5,256
42007ab8:	cc3e                	sw	a5,24(sp)
42007aba:	ce3e                	sw	a5,28(sp)
# 	ESP_ERROR_CHECK(usb_serial_jtag_driver_install(&usb_config));
42007abc:	0828                	add	a0,sp,24
42007abe:	b47fe0ef          	jal	42006604 <usb_serial_jtag_driver_install>
42007ac2:	ed25                	bnez	a0,42007b3a <console_init+0xb6>
# 	usb_serial_jtag_vfs_use_driver();
42007ac4:	f72fe0ef          	jal	42006236 <esp_vfs_usb_serial_jtag_use_driver>
# 	fcntl(fileno(stdin), F_SETFL, 0);
42007ac8:	fe388097          	auipc	ra,0xfe388
42007acc:	e5e080e7          	jalr	-418(ra) # 4038f926 <__getreent>
42007ad0:	4148                	lw	a0,4(a0)
42007ad2:	241520ef          	jal	4205a512 <fileno>
42007ad6:	4601                	li	a2,0
42007ad8:	4591                	li	a1,4
42007ada:	8d5fd0ef          	jal	420053ae <fcntl>
# 	fcntl(fileno(stdout), F_SETFL, 0);
42007ade:	fe388097          	auipc	ra,0xfe388
42007ae2:	e48080e7          	jalr	-440(ra) # 4038f926 <__getreent>
42007ae6:	4508                	lw	a0,8(a0)
42007ae8:	22b520ef          	jal	4205a512 <fileno>
42007aec:	4601                	li	a2,0
42007aee:	4591                	li	a1,4
42007af0:	8bffd0ef          	jal	420053ae <fcntl>
# 	const esp_console_config_t console_config = {
42007af4:	c202                	sw	zero,4(sp)
42007af6:	c402                	sw	zero,8(sp)
42007af8:	c602                	sw	zero,12(sp)
42007afa:	c802                	sw	zero,16(sp)
42007afc:	ca02                	sw	zero,20(sp)
42007afe:	10000793          	li	a5,256
42007b02:	c23e                	sw	a5,4(sp)
42007b04:	47a1                	li	a5,8
42007b06:	c43e                	sw	a5,8(sp)
# 	ESP_ERROR_CHECK(esp_console_init(&console_config));
42007b08:	0048                	add	a0,sp,4
42007b0a:	42a020ef          	jal	42009f34 <esp_console_init>
42007b0e:	e921                	bnez	a0,42007b5e <console_init+0xda>
# 	setvbuf(stdin, NULL, _IONBF, 0);
42007b10:	fe388097          	auipc	ra,0xfe388
42007b14:	e16080e7          	jalr	-490(ra) # 4038f926 <__getreent>
42007b18:	4681                	li	a3,0
42007b1a:	4609                	li	a2,2
42007b1c:	4581                	li	a1,0
42007b1e:	4148                	lw	a0,4(a0)
42007b20:	716530ef          	jal	4205b236 <setvbuf>
# 	linenoiseHistorySetMaxLen(20);
42007b24:	4551                	li	a0,20
42007b26:	2a1030ef          	jal	4200b5c6 <linenoiseHistorySetMaxLen>
# 	linenoiseSetDumbMode(1);
42007b2a:	4505                	li	a0,1
42007b2c:	3fb020ef          	jal	4200a726 <linenoiseSetDumbMode>
# 	register_core_cli_commands();
42007b30:	db1ff0ef          	jal	420078e0 <register_core_cli_commands>
# }
42007b34:	50b2                	lw	ra,44(sp)
42007b36:	6145                	add	sp,sp,48
42007b38:	8082                	ret
# 	ESP_ERROR_CHECK(usb_serial_jtag_driver_install(&usb_config));
42007b3a:	3c074737          	lui	a4,0x3c074
42007b3e:	78870713          	add	a4,a4,1928 # 3c074788 <_flash_rodata_start+0x4668>
42007b42:	3c07e6b7          	lui	a3,0x3c07e
42007b46:	a2868693          	add	a3,a3,-1496 # 3c07da28 <__func__.1>
42007b4a:	04800613          	li	a2,72
42007b4e:	3c0745b7          	lui	a1,0x3c074
42007b52:	68858593          	add	a1,a1,1672 # 3c074688 <_flash_rodata_start+0x4568>
42007b56:	fe385097          	auipc	ra,0xfe385
42007b5a:	e98080e7          	jalr	-360(ra) # 4038c9ee <_esp_error_check_failed>
# 	ESP_ERROR_CHECK(esp_console_init(&console_config));
42007b5e:	3c074737          	lui	a4,0x3c074
42007b62:	7b470713          	add	a4,a4,1972 # 3c0747b4 <_flash_rodata_start+0x4694>
42007b66:	3c07e6b7          	lui	a3,0x3c07e
42007b6a:	a2868693          	add	a3,a3,-1496 # 3c07da28 <__func__.1>
42007b6e:	05400613          	li	a2,84
42007b72:	3c0745b7          	lui	a1,0x3c074
42007b76:	68858593          	add	a1,a1,1672 # 3c074688 <_flash_rodata_start+0x4568>
42007b7a:	fe385097          	auipc	ra,0xfe385
42007b7e:	e74080e7          	jalr	-396(ra) # 4038c9ee <_esp_error_check_failed>

42007b82 <console_task>:
# 
# /* Waits for Enter while logs are flowing normally; switches into CLI mode
#  * once it sees one, then goes back to waiting once run_cli() returns. */
# void console_task(void *arg)
# {
42007b82:	1141                	add	sp,sp,-16
42007b84:	c606                	sw	ra,12(sp)
42007b86:	a019                	j	42007b8c <console_task+0xa>
# 	while (1) {
# 		int c = fgetc(stdin);
# 		if (c == '\n' || c == '\r') {
# 			run_cli();
42007b88:	e3dff0ef          	jal	420079c4 <run_cli>
# 		int c = fgetc(stdin);
42007b8c:	fe388097          	auipc	ra,0xfe388
42007b90:	d9a080e7          	jalr	-614(ra) # 4038f926 <__getreent>
42007b94:	4148                	lw	a0,4(a0)
42007b96:	167520ef          	jal	4205a4fc <fgetc>
# 		if (c == '\n' || c == '\r') {
42007b9a:	47a9                	li	a5,10
42007b9c:	fef506e3          	beq	a0,a5,42007b88 <console_task+0x6>
42007ba0:	47b5                	li	a5,13
42007ba2:	fef515e3          	bne	a0,a5,42007b8c <console_task+0xa>
42007ba6:	b7cd                	j	42007b88 <console_task+0x6>

