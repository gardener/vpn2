#!/bin/sh

cmd=$1

if [ "$cmd" == "on" ]; then
    iptables -A INPUT -m state --state RELATED,ESTABLISHED -i tun0 -j ACCEPT
    iptables -A INPUT -i tun0 -j DROP
elif [ "$cmd" == "off" ]; then
    iptables -D INPUT -m state --state RELATED,ESTABLISHED -i tun0 -j ACCEPT
    iptables -D INPUT -i tun0 -j DROP
else
    echo "usage: $0 [on|off]"
fi
