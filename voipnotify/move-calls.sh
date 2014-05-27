 #!/bin/sh

# if [ -e /home/voipnotify/calls/monitoring-message.gsm ]
# then
	mv -f /home/voipnotify/calls/monitoring-message.gsm /var/lib/asterisk/sounds/monitoring-message.gsm
	sleep 2
	mv /home/voipnotify/calls/*.call /var/spool/asterisk/outgoing/
# fi
