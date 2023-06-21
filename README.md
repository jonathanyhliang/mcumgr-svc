# MCU Management Service

## Overview
This repository implements a [go-kit](https://github.com/go-kit/kit) based microservice which prvoides RESTful APIs to conduct firmware update of MCU over serial port. To accomplish the firmware update, the MCU should either have its pre-programmed application code with [mcumgr subsystem](https://docs.zephyrproject.org/latest/services/device_mgmt/mcumgr.html) ready, or have its pre-programmed bootloader [mcuboot](https://docs.mcuboot.com/) with **mcumgr** serial recovery enabled. In addition, the service should run together with RabbitMQ broker since it relies on the counterpart service to ping a message through AMQP to notify the handover of serial port previllage.


