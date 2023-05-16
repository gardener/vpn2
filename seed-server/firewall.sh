#!/bin/sh

cmd=$1
dev=$2

iptables() {
  # execute all commands for IPv4 and IPv6
  command iptables "$@"
  command ip6tables "$@"
}

if [ "$cmd" == "on" ]; then
    iptables -A INPUT -m state --state RELATED,ESTABLISHED -i $dev -j ACCEPT
    iptables -A INPUT -i $dev -j DROP
elif [ "$cmd" == "off" ]; then
    iptables -D INPUT -m state --state RELATED,ESTABLISHED -i $dev -j ACCEPT
    iptables -D INPUT -i $dev -j DROP
else
    echo "usage: $0 [on|off] dev"
fi
