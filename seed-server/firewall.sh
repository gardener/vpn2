#!/bin/sh

cmd=$1
dev=$2
service_network=$3
pod_network=$4

if iptables-legacy -L >/dev/null && ip6tables-legacy -L >/dev/null ; then
  echo "using iptables backend legacy" 
  backend="-legacy"
elif iptables-nft -L >/dev/null && ip6tables-nft -L >/dev/null ; then
  echo "using iptables backend nft" 
  backend="-nft"
else
  echo "iptables seems not to be supported."
  exit 1
fi

iptables() {
  # execute all commands for IPv4 and IPv6
  command iptables$backend "$@"
  command ip6tables$backend "$@"
}

if [ "$cmd" = "on" ]; then
    iptables -A INPUT -m state --state RELATED,ESTABLISHED -i $dev -j ACCEPT
    iptables -A INPUT -i $dev -j DROP
    ip route add ${service_network} dev $dev
    ip route add ${pod_network} dev $dev
    # FIXME add NODE_NETWORK
elif [ "$cmd" = "off" ]; then
    iptables -D INPUT -m state --state RELATED,ESTABLISHED -i $dev -j ACCEPT
    iptables -D INPUT -i $dev -j DROP
else
    echo "usage: $0 [on|off] dev"
fi
