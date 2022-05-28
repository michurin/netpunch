#!/bin/bash

## Self documented, ready to use example

set -x # Remove to disable debugging

## SETTINGS

# Assume we start server on the host you-host-with-public-ip.net by the command like that:
# netpunch -secret x -local :10001
# You have to open this port. For example by iptables rule:
# -A INPUT -p udp -m udp --dport 10001 -j ACCEPT
SERVER='you-host-with-public-ip.net:10001'

# Shared secret, has to be the same on all nodes: peers and control one
# You are also able to read secret from file, using -secret-file option
SECRET='Secret'

# Local port, you are free to change it
LPORT='10000'

# On B node you have to set ROLE=b and swap values of LOCALIP and REMOTEIP
ROLE='a'
LOCALIP='192.168.2.1' # Of cause you are free to use and IP like 10.8.8.8 etc.
REMOTEIP='192.168.2.2'

NETPUNCH='./netpunch'

# You may want to setup sudo like that:
# user ALL=(root) NOPASSWD: /usr/bin/openvpn
OPENVPN='sudo /usr/bin/openvpn'
# Shared secret for OpenVPN:
# openvpn --genkey secret secret.key
OPENVPNSECRET='secret.key'

## END OF SETTINGS

test -f $OPENVPNSECRET || {
    echo "OpenVPN secret not found: $OPENVPNSECRET"
    exit 1
}

while :
do
    params=($($NETPUNCH -peer $ROLE -secret $SECRET -local :$LPORT -remote $SERVER)) || {
        echo "Error code: $?: sleep and retry..."
        sleep 30
        continue
    }

    test 'LADDR/LHOST/LPORT/RADDR/RHOST/RPORT:' = "${params[0]}" || {
        echo "Wrong result: ${params[@]}: sleep and retry..."
        sleep 30
        continue
    }

    lport=${params[3]}
    rhost=${params[5]}
    rport=${params[6]}

    echo "******* GOT LPORT=$lport RHOST=$rhost RPORT=$rport *******"

    $OPENVPN \
        --remote $rhost --rport $rport \
        --lport $lport \
        --proto udp --dev tun \
        --ifconfig $LOCALIP $REMOTEIP \
        --auth-nocache --secret $OPENVPNSECRET --auth SHA256 --cipher AES-256-CBC \
        --ping 10 --ping-exit 40 \
        --verb 3
done
