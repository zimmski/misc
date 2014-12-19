 #!/bin/sh

# if [ -e /home/voipnotify/calls/monitoring-message.gsm ]
# then
	rm /var/lib/asterisk/sounds/monitoring-message.gsm
	cp /home/voipnotify/calls/monitoring-message.gsm /var/lib/asterisk/sounds/monitoring-message.gsm
	sleep 2
	for f in /home/voipnotify/calls/*.call; do
		[ -e "$f" ] && mv /home/voipnotify/calls/*.call /var/spool/asterisk/outgoing/

		break
	done
# fi
