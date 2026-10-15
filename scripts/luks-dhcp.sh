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

if [ -n "$new_domain_name_servers" ]; then
    {
        [ -n "$new_domain_search" ] && echo "search $new_domain_search"
        [ -z "$new_domain_search" ] && [ -n "$new_domain_name" ] && echo "search $new_domain_name"
        for ns in $new_domain_name_servers; do
            echo "nameserver $ns"
        done
    } > /etc/resolv.conf
fi

echo "Done"
