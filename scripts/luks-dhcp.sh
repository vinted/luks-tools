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

echo "Done"
