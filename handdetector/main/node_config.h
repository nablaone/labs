#pragma once

/*
 * Compile-time configuration for the DFRobot FireBeetle 2 ESP32-S3-U
 * (DFR0975-U) with its OV2640 camera: pin assignments and firmware version.
 *
 * Pin map from the DFRobot wiki (SKU_DFR0975 "CAM Interface" table) and
 * the arduino-esp32 dfrobot_firebeetle2_esp32s3 variant.
 */

#include "driver/gpio.h"

#define FIRMWARE_VERSION "0.1.0 (" __DATE__ ")"

/* Onboard LED (labelled 21/D13), active-high. */
#define LED_GPIO     GPIO_NUM_21
#define LED_ON_LEVEL 1

/*
 * Shared I2C bus: the AXP313A power-management chip (camera supplies) and
 * the camera's SCCB control port both live on GPIO1 (SDA) / GPIO2 (SCL).
 * One i2c_master bus on I2C_BUS_PORT is opened by pmic.c and shared --
 * esp32-camera borrows it by port number (pin_sccb_sda = -1).
 */
#define I2C_SDA_GPIO GPIO_NUM_1
#define I2C_SCL_GPIO GPIO_NUM_2
#define I2C_BUS_PORT 0
#define AXP313A_ADDR 0x36

/*
 * OV2640 DVP interface. PWDN is hard-wired low (always on) and RESET is
 * pulled up to DOVDD, so neither has a GPIO. GPIO45/46 are strapping pins
 * but are only driven/read after boot, so they're fine here.
 * Y2..Y9 are the 8 data bits (the sensor's D2..D9 of its 10-bit bus).
 */
#define CAM_PIN_PWDN  -1
#define CAM_PIN_RESET -1
#define CAM_PIN_XCLK  45
#define CAM_PIN_SIOD  I2C_SDA_GPIO
#define CAM_PIN_SIOC  I2C_SCL_GPIO
#define CAM_PIN_Y9    48
#define CAM_PIN_Y8    46
#define CAM_PIN_Y7    8
#define CAM_PIN_Y6    7
#define CAM_PIN_Y5    4
#define CAM_PIN_Y4    41
#define CAM_PIN_Y3    40
#define CAM_PIN_Y2    39
#define CAM_PIN_VSYNC 6
#define CAM_PIN_HREF  42
#define CAM_PIN_PCLK  5
