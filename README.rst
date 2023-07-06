MCU Management Service
######################

Overview
########
``mcumgr-svc`` implements a `go-kit <https://github.com/go-kit/kit>`_ based microservice which
prvoides RESTful APIs to conduct firmware update of MCU over the serial port. The frontend of
the service is a `Hawkbit <https://www.eclipse.org/hawkbit/>`_ compliant FOTA client which
polls and launches FOTA processes by RESTful APIs a ``Hawkbit FOTA Server`` offers to retrive
an FOTA deployment. The serial backend of the service has ``upload`` and ``reset`` methods of
`mcumgr <https://github.com/apache/mynewt-mcumgr>`_ implemented to update a serial connected
device which runs `mcuboot <https://github.com/jonathanyhliang/mcuboot>`_ bootloader in serial
recovery mode, and make the device boot into application code once the update was successful.

Building and Running
####################

The primary use of ``mcumgr-svc`` is run with `hawkbit-svc <https://github.com/jonathanyhliang/hawkbit-fota>`_
and `slcan-svc <https://github.com/jonathanyhliang/slcan-svc>`_. Refer to
`demo-svc <https://github.com/jonathanyhliang/demo-svc>`_ for the full picture of how things work.
