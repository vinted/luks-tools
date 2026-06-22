#!/bin/sh

echo "Starting luks-dhcp hook"

if [ -z "$new_ip_address" ]
then
    echo "Missing required env variables. Exiting"
    exit
fi
prefix=`ipcalc -p $new_ip_address $new_subnet_mask | sed 's/PREFIX=//'`
ip addr add $new_ip_address/$prefix dev $interface
ip route add default via $new_routers dev $interface

# Populate resolv.conf from the DHCP lease. Without this the initramfs relies on
# the resolv.conf baked in at build time, which goes stale when DNS servers are
# renumbered and leaves luks-key unable to resolve the Vault hostname.
resolv_conf="${RESOLV_CONF:-/etc/resolv.conf}"
if [ -n "$new_domain_name_servers" ]
then
    echo "Writing $resolv_conf from DHCP lease"
    : > "$resolv_conf"
    for ns in $new_domain_name_servers
    do
        echo "nameserver $ns" >> "$resolv_conf"
    done
    if [ -n "$new_domain_search" ]
    then
        echo "search $new_domain_search" >> "$resolv_conf"
    elif [ -n "$new_domain_name" ]
    then
        echo "search $new_domain_name" >> "$resolv_conf"
    fi
fi

echo "Done"
